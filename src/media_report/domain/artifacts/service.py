from __future__ import annotations

import json
from datetime import UTC, datetime
from pathlib import Path

from pydantic import ValidationError

from media_report.core.constants import ARTIFACT_SUFFIX
from media_report.core.errors import (
  ArtifactConflictError,
  ArtifactMetadataError,
  StagePrerequisiteError,
)
from media_report.domain.artifacts.entities import (
  ArtifactPlan,
  PipelineArtifactMetadata,
  PipelineMetadata,
  PipelineSourceMetadata,
  PipelineStage,
  PipelineStageDecision,
  PipelineStageMetadata,
  PipelineStageStatus,
  PipelineTranscriptionMetadata,
  PipelineWorkflowMetadata,
  StageDecision,
  StageErrorSummary,
)
from media_report.domain.media.entities import MediaSource
from media_report.domain.transcription.entities import TranscriptionResult


class ArtifactPlanner:
  def plan(self, media_path: Path) -> ArtifactPlan:
    root_dir = media_path.parent / f"{media_path.stem}{ARTIFACT_SUFFIX}"
    return ArtifactPlan(
      root_dir=root_dir,
      metadata_json=root_dir / "metadata.json",
      pipeline_log=root_dir / "pipeline.log",
      audio_extracted=root_dir / "audio_extracted.wav",
      audio_normalized=root_dir / "audio_normalized.wav",
      transcript_raw=root_dir / "transcript_raw.txt",
      transcript_segments=root_dir / "transcript_segments.json",
      transcript_clean=root_dir / "transcript_clean.md",
      prompt_used=root_dir / "prompt_used.md",
      llm_response_raw=root_dir / "llm_response_raw.txt",
      report_markdown=root_dir / "report.md",
      report_pdf=root_dir / "report.pdf",
    )

  def prepare_new(self, media_path: Path) -> ArtifactPlan:
    artifact_plan = self.plan(media_path)
    if artifact_plan.root_dir.exists():
      raise ArtifactConflictError(
        f"Artifact directory already exists for '{media_path.name}': "
        f"{artifact_plan.root_dir}. Use --resume to reuse it. "
        "--overwrite is still accepted as a deprecated alias."
      )
    artifact_plan.root_dir.mkdir(parents=True, exist_ok=False)
    return artifact_plan

  def bootstrap_metadata(
    self,
    source: MediaSource,
    artifact_plan: ArtifactPlan,
    template_name: str,
    llm_provider: str,
    llm_model: str,
    output_format: str,
    language: str | None,
    selected_stages: tuple[PipelineStage, ...],
  ) -> PipelineMetadata:
    generated_at = datetime.now(UTC).isoformat()
    selected_stage_set = set(selected_stages)
    stages = {
      stage: self._build_stage_metadata(
        generated_at=generated_at,
        selected=stage in selected_stage_set,
      )
      for stage in PipelineStage
    }
    return PipelineMetadata(
      schema_version=2,
      generated_at=generated_at,
      source=PipelineSourceMetadata(
        path=source.path,
        kind=source.kind.value,
      ),
      artifacts=PipelineArtifactMetadata(
        root_dir=artifact_plan.root_dir,
        metadata_json=artifact_plan.metadata_json,
        pipeline_log=artifact_plan.pipeline_log,
        audio_extracted=artifact_plan.audio_extracted,
        audio_normalized=artifact_plan.audio_normalized,
        transcript_raw=artifact_plan.transcript_raw,
        transcript_segments=artifact_plan.transcript_segments,
        transcript_clean=artifact_plan.transcript_clean,
        prompt_used=artifact_plan.prompt_used,
        llm_response_raw=artifact_plan.llm_response_raw,
        report_markdown=artifact_plan.report_markdown,
        report_pdf=artifact_plan.report_pdf,
      ),
      workflow=self._build_workflow_metadata(
        template_name=template_name,
        llm_provider=llm_provider,
        llm_model=llm_model,
        output_format=output_format,
        language=language,
        selected_stages=selected_stages,
      ),
      stages=stages,
    )

  def update_workflow(
    self,
    metadata: PipelineMetadata,
    *,
    template_name: str,
    llm_provider: str,
    llm_model: str,
    output_format: str,
    language: str | None,
    selected_stages: tuple[PipelineStage, ...],
  ) -> PipelineMetadata:
    return metadata.model_copy(
      update={
        "workflow": self._build_workflow_metadata(
          template_name=template_name,
          llm_provider=llm_provider,
          llm_model=llm_model,
          output_format=output_format,
          language=language,
          selected_stages=selected_stages,
        )
      }
    )

  def initialize_log(self, artifact_root: Path, *, metadata_schema_version: int) -> None:
    log_path = artifact_root / "pipeline.log"
    bootstrap_line = f"media-report metadata initialized (schema v{metadata_schema_version})\n"
    if not log_path.exists():
      log_path.write_text(bootstrap_line, encoding="utf-8")
      return
    with log_path.open("a", encoding="utf-8") as handle:
      handle.write(bootstrap_line)

  def ensure_log(self, artifact_root: Path) -> None:
    log_path = artifact_root / "pipeline.log"
    if log_path.exists():
      return
    log_path.write_text(
      "media-report pipeline log initialized for existing artifacts\n",
      encoding="utf-8",
    )

  def append_stage_decisions(
    self,
    artifact_root: Path,
    stage_decisions: tuple[StageDecision, ...],
  ) -> None:
    log_path = artifact_root / "pipeline.log"
    with log_path.open("a", encoding="utf-8") as handle:
      for decision in stage_decisions:
        handle.write(f"{decision.stage.value}: {decision.decision.value} - {decision.reason}\n")

  def append_log_event(self, artifact_root: Path, message: str) -> None:
    log_path = artifact_root / "pipeline.log"
    with log_path.open("a", encoding="utf-8") as handle:
      handle.write(f"{message}\n")

  def mark_stage_running(
    self,
    metadata: PipelineMetadata,
    *,
    stage: PipelineStage,
  ) -> PipelineMetadata:
    now = self._now()
    current = metadata.stages[stage]
    updated_stage = current.model_copy(
      update={
        "status": PipelineStageStatus.RUNNING,
        "resumable": False,
        "started_at": now,
        "finished_at": None,
        "updated_at": now,
        "error": None,
      }
    )
    return metadata.model_copy(update={"stages": {**metadata.stages, stage: updated_stage}})

  def mark_stage_completed(
    self,
    metadata: PipelineMetadata,
    *,
    stage: PipelineStage,
  ) -> PipelineMetadata:
    now = self._now()
    current = metadata.stages[stage]
    updated_stage = current.model_copy(
      update={
        "status": PipelineStageStatus.COMPLETED,
        "resumable": True,
        "started_at": current.started_at or now,
        "finished_at": now,
        "updated_at": now,
        "error": None,
      }
    )
    return metadata.model_copy(update={"stages": {**metadata.stages, stage: updated_stage}})

  def mark_stage_failed(
    self,
    metadata: PipelineMetadata,
    *,
    stage: PipelineStage,
    error: StageErrorSummary,
  ) -> PipelineMetadata:
    now = self._now()
    current = metadata.stages[stage]
    updated_stage = current.model_copy(
      update={
        "status": PipelineStageStatus.FAILED,
        "resumable": True,
        "started_at": current.started_at or now,
        "finished_at": now,
        "updated_at": now,
        "error": error,
      }
    )
    return metadata.model_copy(update={"stages": {**metadata.stages, stage: updated_stage}})

  def reset_stages_to_planned(
    self,
    metadata: PipelineMetadata,
    *,
    stages: tuple[PipelineStage, ...],
  ) -> PipelineMetadata:
    updated_stages = dict(metadata.stages)
    for stage in stages:
      current = metadata.stages[stage]
      updated_stages[stage] = current.model_copy(
        update={
          "status": PipelineStageStatus.PLANNED,
          "resumable": True,
          "started_at": None,
          "finished_at": None,
          "updated_at": self._now(),
          "error": None,
        }
      )
    return metadata.model_copy(update={"stages": updated_stages})

  def record_transcription(
    self,
    metadata: PipelineMetadata,
    *,
    result: TranscriptionResult,
    completed_at: str | None = None,
  ) -> PipelineMetadata:
    return metadata.model_copy(
      update={
        "transcription": PipelineTranscriptionMetadata(
          provider=result.provider,
          model=result.model,
          requested_language=result.requested_language,
          detected_language=result.detected_language,
          duration_ms=result.duration_ms,
          completed_at=completed_at or self._now(),
          device_preference=result.device_preference,
          effective_device=result.effective_device,
          device_fallback_reason=result.device_fallback_reason,
        )
      }
    )

  @staticmethod
  def _build_workflow_metadata(
    *,
    template_name: str,
    llm_provider: str,
    llm_model: str,
    output_format: str,
    language: str | None,
    selected_stages: tuple[PipelineStage, ...],
  ) -> PipelineWorkflowMetadata:
    return PipelineWorkflowMetadata(
      template_name=template_name,
      llm_provider=llm_provider,
      llm_model=llm_model,
      output_format=output_format,
      language=language,
      selected_stages=selected_stages,
    )

  @staticmethod
  def _build_stage_metadata(*, generated_at: str, selected: bool) -> PipelineStageMetadata:
    if selected:
      return PipelineStageMetadata(
        status=PipelineStageStatus.PLANNED,
        resumable=True,
        started_at=None,
        finished_at=None,
        updated_at=generated_at,
        error=None,
      )
    return PipelineStageMetadata(
      status=PipelineStageStatus.SKIPPED,
      resumable=True,
      started_at=None,
      finished_at=generated_at,
      updated_at=generated_at,
      error=None,
    )

  @staticmethod
  def _now() -> str:
    return datetime.now(UTC).isoformat()


class ArtifactRootValidator:
  def validate(
    self,
    *,
    source: MediaSource,
    artifact_plan: ArtifactPlan,
    metadata: PipelineMetadata,
  ) -> None:
    expected_paths = {
      "root_dir": artifact_plan.root_dir,
      "metadata_json": artifact_plan.metadata_json,
      "pipeline_log": artifact_plan.pipeline_log,
      "audio_extracted": artifact_plan.audio_extracted,
      "audio_normalized": artifact_plan.audio_normalized,
      "transcript_raw": artifact_plan.transcript_raw,
      "transcript_segments": artifact_plan.transcript_segments,
      "transcript_clean": artifact_plan.transcript_clean,
      "prompt_used": artifact_plan.prompt_used,
      "llm_response_raw": artifact_plan.llm_response_raw,
      "report_markdown": artifact_plan.report_markdown,
      "report_pdf": artifact_plan.report_pdf,
    }

    if Path(metadata.source.path) != source.path:
      raise ArtifactMetadataError(f"Artifact metadata source path does not match '{source.path}'.")
    if metadata.source.kind != source.kind.value:
      raise ArtifactMetadataError(
        f"Artifact metadata kind does not match '{source.kind.value}' for '{source.path.name}'."
      )

    for field_name, expected_path in expected_paths.items():
      persisted_path = getattr(metadata.artifacts, field_name)
      if persisted_path != expected_path:
        raise ArtifactMetadataError(
          f"Artifact metadata path mismatch for '{field_name}': "
          f"expected '{expected_path}', "
          f"found '{persisted_path}'."
        )

    for stage in PipelineStage:
      if metadata.stages[stage].status != PipelineStageStatus.COMPLETED:
        continue

      if missing_outputs := self.missing_outputs_for_stage(
        artifact_plan=artifact_plan,
        stage=stage,
      ):
        missing_names = ", ".join(path.name for path in missing_outputs)
        raise ArtifactMetadataError(
          f"Stage '{stage.value}' is marked completed but "
          "required artifacts are missing: "
          f"{missing_names}."
        )
      if stage == PipelineStage.TRANSCRIBE:
        self._validate_transcription_outputs(artifact_plan=artifact_plan)
      if stage == PipelineStage.REPORT:
        self._validate_report_outputs(artifact_plan=artifact_plan)

  def _validate_transcription_outputs(self, *, artifact_plan: ArtifactPlan) -> None:
    try:
      payload = json.loads(artifact_plan.transcript_segments.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
      raise ArtifactMetadataError(
        "Stage 'transcribe' is marked completed but 'transcript_segments.json' is not valid JSON."
      ) from exc

    try:
      result = TranscriptionResult.model_validate(payload)
    except (KeyError, TypeError, ValueError, ValidationError) as exc:
      raise ArtifactMetadataError(
        "Stage 'transcribe' is marked completed but "
        "'transcript_segments.json' does not match the structured contract."
      ) from exc

    raw_text = artifact_plan.transcript_raw.read_text(encoding="utf-8").strip()
    if raw_text != result.raw_text.strip():
      raise ArtifactMetadataError(
        "Stage 'transcribe' is marked completed but transcript artifacts are inconsistent."
      )

  def _validate_report_outputs(self, *, artifact_plan: ArtifactPlan) -> None:
    issue = self.report_completion_issue(artifact_plan=artifact_plan)
    if issue is None:
      return
    raise ArtifactMetadataError(
      "Stage 'report' is marked completed but "
      f"{issue} Re-run reporting to regenerate the report artifacts."
    )

  def report_completion_issue(
    self,
    *,
    artifact_plan: ArtifactPlan,
  ) -> str | None:
    if missing_outputs := self.missing_outputs_for_stage(
      artifact_plan=artifact_plan,
      stage=PipelineStage.REPORT,
    ):
      missing_names = ", ".join(path.name for path in missing_outputs)
      return f"required artifacts are missing: {missing_names}."

    try:
      prompt_text = artifact_plan.prompt_used.read_text(encoding="utf-8")
      llm_response = artifact_plan.llm_response_raw.read_text(encoding="utf-8")
      report_text = artifact_plan.report_markdown.read_text(encoding="utf-8")
    except OSError as exc:
      raise ArtifactMetadataError(
        "Stage 'report' is marked completed but report artifacts could not be read."
      ) from exc

    if not prompt_text.strip():
      return "'prompt_used.md' is empty."
    if not llm_response.strip():
      return "'llm_response_raw.txt' is empty."
    if report_text != _normalize_report_text(llm_response):
      return "'report.md' is inconsistent with 'llm_response_raw.txt'."
    return None

  @staticmethod
  def outputs_for_stage(
    *,
    artifact_plan: ArtifactPlan,
    stage: PipelineStage,
  ) -> tuple[Path, ...]:
    match stage:
      case PipelineStage.EXTRACT_AUDIO:
        return (artifact_plan.audio_extracted,)
      case PipelineStage.NORMALIZE_AUDIO:
        return (artifact_plan.audio_normalized,)
      case PipelineStage.TRANSCRIBE:
        return (artifact_plan.transcript_raw, artifact_plan.transcript_segments)
      case PipelineStage.REPORT:
        return (
          artifact_plan.prompt_used,
          artifact_plan.llm_response_raw,
          artifact_plan.report_markdown,
        )
      case PipelineStage.PDF:
        return (artifact_plan.report_pdf,)

  def missing_outputs_for_stage(
    self,
    *,
    artifact_plan: ArtifactPlan,
    stage: PipelineStage,
  ) -> tuple[Path, ...]:
    return tuple(
      output_path
      for output_path in self.outputs_for_stage(artifact_plan=artifact_plan, stage=stage)
      if not output_path.exists()
    )


def _normalize_report_text(text: str) -> str:
  return text if text.endswith("\n") else f"{text}\n"


class PipelineStatePlanner:
  STAGE_SEQUENCE = tuple(PipelineStage)

  def select_stages(
    self,
    *,
    only_transcribe: bool,
    only_report: bool,
  ) -> tuple[PipelineStage, ...]:
    if only_transcribe and only_report:
      raise StagePrerequisiteError("Choose either --only-transcribe or --only-report, not both.")
    if only_report:
      return (PipelineStage.REPORT, PipelineStage.PDF)
    if only_transcribe:
      return (
        PipelineStage.EXTRACT_AUDIO,
        PipelineStage.NORMALIZE_AUDIO,
        PipelineStage.TRANSCRIBE,
      )
    return self.STAGE_SEQUENCE

  def plan_new(self, requested_stages: tuple[PipelineStage, ...]) -> tuple[StageDecision, ...]:
    self._ensure_new_run_can_start(requested_stages)
    requested_stage_set = set(requested_stages)
    return tuple(
      StageDecision(
        stage=stage,
        decision=(
          PipelineStageDecision.PLANNED
          if stage in requested_stage_set
          else PipelineStageDecision.SKIPPED
        ),
        reason=(
          "Stage selected for this invocation."
          if stage in requested_stage_set
          else "Stage omitted from this invocation."
        ),
      )
      for stage in self.STAGE_SEQUENCE
    )

  def plan_resume(
    self,
    *,
    metadata: PipelineMetadata,
    requested_stages: tuple[PipelineStage, ...],
    force_stages: set[PipelineStage] | None = None,
  ) -> tuple[StageDecision, ...]:
    furthest_requested_index = max(self.STAGE_SEQUENCE.index(stage) for stage in requested_stages)
    requested_stage_set = set(requested_stages)
    force_stages = force_stages or set()
    satisfied_stages: set[PipelineStage] = set()
    decisions: list[StageDecision] = []

    for index, stage in enumerate(self.STAGE_SEQUENCE):
      if index > furthest_requested_index:
        decisions.append(
          StageDecision(
            stage=stage,
            decision=PipelineStageDecision.SKIPPED,
            reason="Stage omitted from this invocation.",
          )
        )
        continue

      stage_metadata = metadata.stages[stage]
      if stage_metadata.status == PipelineStageStatus.COMPLETED and stage not in force_stages:
        decisions.append(
          StageDecision(
            stage=stage,
            decision=PipelineStageDecision.REUSED,
            reason="Using completed artifacts from existing metadata.",
          )
        )
        satisfied_stages.add(stage)
        continue

      if stage in requested_stage_set:
        missing_prerequisite = self._first_missing_prerequisite(
          stage=stage,
          satisfied_stages=satisfied_stages,
        )
        if missing_prerequisite is not None:
          raise StagePrerequisiteError(
            f"Cannot plan '{stage.value}' because prerequisite "
            f"'{missing_prerequisite.value}' is not completed "
            "in the existing artifacts."
          )
        decisions.append(
          StageDecision(
            stage=stage,
            decision=PipelineStageDecision.PLANNED,
            reason="Stage selected for this invocation.",
          )
        )
        satisfied_stages.add(stage)
        continue

      blocking_stage = self._first_requested_stage_after(
        index=index,
        requested_stages=requested_stages,
      )
      if blocking_stage is None:
        decisions.append(
          StageDecision(
            stage=stage,
            decision=PipelineStageDecision.SKIPPED,
            reason="Stage omitted from this invocation.",
          )
        )
        continue
      raise StagePrerequisiteError(
        f"Cannot plan '{blocking_stage.value}' because prerequisite '{stage.value}' "
        "is not completed. Resume after the earlier stages finish successfully."
      )

    return tuple(decisions)

  def _ensure_new_run_can_start(self, requested_stages: tuple[PipelineStage, ...]) -> None:
    first_requested_stage = requested_stages[0]
    if first_requested_stage == PipelineStage.EXTRACT_AUDIO:
      return
    raise StagePrerequisiteError(
      f"Cannot start a fresh pipeline at '{first_requested_stage.value}'. "
      "Create the earlier artifacts first, then rerun with --resume."
    )

  def _first_missing_prerequisite(
    self,
    *,
    stage: PipelineStage,
    satisfied_stages: set[PipelineStage],
  ) -> PipelineStage | None:
    stage_index = self.STAGE_SEQUENCE.index(stage)
    return next(
      (
        prior_stage
        for prior_stage in self.STAGE_SEQUENCE[:stage_index]
        if prior_stage not in satisfied_stages
      ),
      None,
    )

  def _first_requested_stage_after(
    self,
    *,
    index: int,
    requested_stages: tuple[PipelineStage, ...],
  ) -> PipelineStage | None:
    return next(
      (stage for stage in requested_stages if self.STAGE_SEQUENCE.index(stage) > index),
      None,
    )
