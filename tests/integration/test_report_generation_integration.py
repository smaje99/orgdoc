from __future__ import annotations

from pathlib import Path

from media_report.application.reporting import (
  GenerateReportRequest,
  PromptRenderService,
  ReportGenerationService,
)
from media_report.domain.artifacts.entities import PipelineStage
from media_report.domain.artifacts.service import ArtifactPlanner
from media_report.domain.media.entities import MediaSource
from media_report.domain.reporting.ports import DocumentRenderer, LLMProvider
from media_report.domain.transcription.entities import (
  TranscriptionResult,
  TranscriptionSegment,
)
from media_report.infrastructure.filesystem.metadata_repository import (
  JsonPipelineMetadataRepository,
)
from media_report.infrastructure.filesystem.scanner import FileSystemMediaScanner
from media_report.infrastructure.filesystem.transcription_repository import (
  JsonTranscriptionArtifactRepository,
)
from media_report.infrastructure.resources.templates import PackagePromptTemplateRepository


class FakeLLMProvider(LLMProvider):
  def __init__(self, response: str) -> None:
    self.response = response
    self.calls: list[tuple[str, str]] = []

  def generate(self, prompt: str, *, model: str) -> str:
    self.calls.append((prompt, model))
    return self.response


class FakeDocumentRenderer(DocumentRenderer):
  def __init__(self) -> None:
    self.calls: list[tuple[Path, Path]] = []

  def render(self, markdown_path: Path, pdf_path: Path) -> None:
    self.calls.append((markdown_path, pdf_path))
    pdf_path.write_text("%PDF-1.4", encoding="utf-8")


def build_transcription_result() -> TranscriptionResult:
  return TranscriptionResult(
    provider="faster-whisper",
    model="small",
    requested_language=None,
    detected_language="en",
    segments=(
      TranscriptionSegment(
        index=0,
        start_seconds=0.0,
        end_seconds=1.0,
        text="project status and next steps",
      ),
    ),
    duration_ms=80,
  )


def write_transcribed_artifact_root(media_path: Path) -> Path:
  planner = ArtifactPlanner()
  artifact_plan = planner.prepare_new(media_path)
  source = FileSystemMediaScanner().classify(media_path)
  metadata = planner.bootstrap_metadata(
    source=MediaSource(path=media_path, kind=source.kind),
    artifact_plan=artifact_plan,
    template_name="technical_report",
    llm_provider="ollama",
    llm_model="llama3.1",
    output_format="pdf",
    language=None,
    selected_stages=tuple(PipelineStage),
  )
  artifact_plan.audio_extracted.write_text("audio", encoding="utf-8")
  artifact_plan.audio_normalized.write_text("normalized", encoding="utf-8")
  JsonTranscriptionArtifactRepository().write(
    result=build_transcription_result(),
    transcript_raw_path=artifact_plan.transcript_raw,
    transcript_segments_path=artifact_plan.transcript_segments,
  )
  metadata = planner.mark_stage_completed(metadata, stage=PipelineStage.EXTRACT_AUDIO)
  metadata = planner.mark_stage_completed(metadata, stage=PipelineStage.NORMALIZE_AUDIO)
  metadata = planner.record_transcription(metadata, result=build_transcription_result())
  metadata = planner.mark_stage_completed(metadata, stage=PipelineStage.TRANSCRIBE)
  JsonPipelineMetadataRepository().write(metadata)
  planner.initialize_log(artifact_plan.root_dir, metadata_schema_version=metadata.schema_version)
  return artifact_plan.root_dir


def test_generate_report_uses_packaged_template_and_persists_markdown(tmp_path: Path) -> None:
  media_path = tmp_path / "session.mp3"
  media_path.write_text("audio", encoding="utf-8")
  artifact_root = write_transcribed_artifact_root(media_path)
  provider = FakeLLMProvider("# Technical Report\n\n## Summary\n\nAll good.")
  renderer = FakeDocumentRenderer()
  metadata_repository = JsonPipelineMetadataRepository()
  prompt_renderer = PromptRenderService(
    scanner=FileSystemMediaScanner(),
    metadata_repository=metadata_repository,
    template_repository=PackagePromptTemplateRepository(),
  )
  service = ReportGenerationService(
    prompt_renderer=prompt_renderer,
    metadata_repository=metadata_repository,
    provider_resolver=lambda _provider_name: provider,
    document_renderer=renderer,
  )

  result = service.generate_report(GenerateReportRequest(input_path=artifact_root))

  assert provider.calls
  assert (
    "Create a technical Markdown report with concise, factual language." in result.rendered_prompt
  )
  assert (
    result.response_path.read_text(encoding="utf-8")
    == "# Technical Report\n\n## Summary\n\nAll good."
  )
  assert (
    result.report_path.read_text(encoding="utf-8")
    == "# Technical Report\n\n## Summary\n\nAll good.\n"
  )
  assert renderer.calls == [(artifact_root / "report.md", artifact_root / "report.pdf")]
  assert (artifact_root / "report.pdf").read_text(encoding="utf-8") == "%PDF-1.4"
