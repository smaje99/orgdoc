from __future__ import annotations

from collections.abc import Callable, Sequence
from pathlib import Path
from typing import NoReturn, TypeAlias

from media_report.application.reporting.models import GenerateReportRequest, GenerateReportResult
from media_report.application.reporting.ports import PromptRenderUseCase
from media_report.core.errors import (
  LLMProviderConfigurationError,
  LLMProviderExecutionError,
  LLMProviderOutputError,
  PDFRenderingConfigurationError,
  PDFRenderingExecutionError,
  PDFRenderingOutputError,
  PromptRenderPrerequisiteError,
  ReportArtifactPersistenceError,
)
from media_report.core.redaction import redact_text
from media_report.domain.artifacts.entities import (
  PipelineMetadata,
  PipelineStage,
  PipelineStageDecision,
  PipelineStageStatus,
  StageDecision,
  StageErrorSummary,
)
from media_report.domain.artifacts.ports import PipelineMetadataRepository
from media_report.domain.artifacts.service import ArtifactPlanner
from media_report.domain.reporting.ports import DocumentRenderer, LLMProvider

from .models import RenderPromptRequest

PDFRenderingFailure: TypeAlias = (
  PDFRenderingConfigurationError | PDFRenderingExecutionError | PDFRenderingOutputError
)


class ReportGenerationService:
  def __init__(
    self,
    *,
    prompt_renderer: PromptRenderUseCase,
    metadata_repository: PipelineMetadataRepository,
    provider_resolver: Callable[[str], LLMProvider],
    document_renderer: DocumentRenderer,
    secret_values: Sequence[str] = (),
  ) -> None:
    self._prompt_renderer = prompt_renderer
    self._metadata_repository = metadata_repository
    self._provider_resolver = provider_resolver
    self._document_renderer = document_renderer
    self._artifact_planner = ArtifactPlanner()
    self._secret_values = tuple(secret for secret in secret_values if secret)

  def generate_report(self, request: GenerateReportRequest) -> GenerateReportResult:
    prompt_result = self._prompt_renderer.render_prompt(
      RenderPromptRequest(
        input_path=request.input_path,
        template_name=request.template_name,
        overwrite=request.overwrite,
        workflow_selected_stages=request.workflow_selected_stages,
      )
    )
    metadata = self._update_effective_workflow(prompt_result.final_metadata, request=request)
    self._metadata_repository.write(metadata)

    report_decision = _report_stage_decision(prompt_result.stage_decisions)
    if (
      report_decision.decision == PipelineStageDecision.REUSED
      and metadata.stages[PipelineStage.REPORT].status == PipelineStageStatus.COMPLETED
      and not request.overwrite
    ):
      llm_response = prompt_result.artifacts.llm_response_raw.read_text(encoding="utf-8")
      report_text = prompt_result.artifacts.report_markdown.read_text(encoding="utf-8")
      final_metadata = self._complete_pdf_stage_if_needed(
        metadata=metadata,
        stage_decisions=prompt_result.stage_decisions,
        artifacts=prompt_result.artifacts,
      )
      return self._build_result(
        source=prompt_result.source,
        artifacts=prompt_result.artifacts,
        stage_decisions=prompt_result.stage_decisions,
        final_metadata=final_metadata,
        prompt_path=prompt_result.prompt_path,
        rendered_prompt=prompt_result.rendered_prompt,
        llm_response=llm_response,
        report_text=report_text,
      )

    provider_name = metadata.workflow.llm_provider
    try:
      provider = self._provider_resolver(provider_name)
    except LLMProviderConfigurationError as exc:
      self._mark_failure(
        metadata=metadata,
        artifact_root=prompt_result.artifacts.root_dir,
        exc=exc,
      )
      raise exc.__class__(self._sanitize(str(exc))) from exc

    running_metadata = self._artifact_planner.mark_stage_running(
      metadata,
      stage=PipelineStage.REPORT,
    )
    self._metadata_repository.write(running_metadata)
    self._artifact_planner.append_log_event(
      prompt_result.artifacts.root_dir,
      f"report generation started (provider={provider_name}, model={metadata.workflow.llm_model})",
    )

    try:
      llm_response = provider.generate(
        prompt_result.rendered_prompt,
        model=metadata.workflow.llm_model,
      )
      if not llm_response.strip():
        raise LLMProviderOutputError("LLM provider returned empty output.")

      prompt_result.artifacts.llm_response_raw.write_text(llm_response, encoding="utf-8")
      report_text = _normalize_report_text(llm_response)
      prompt_result.artifacts.report_markdown.write_text(report_text, encoding="utf-8")
    except PromptRenderPrerequisiteError:
      raise
    except OSError as exc:
      wrapped = ReportArtifactPersistenceError(
        f"Could not persist report artifacts in '{prompt_result.artifacts.root_dir}'."
      )
      self._mark_failure(
        metadata=running_metadata,
        artifact_root=prompt_result.artifacts.root_dir,
        exc=wrapped,
      )
      raise wrapped from exc
    except (
      LLMProviderConfigurationError,
      LLMProviderExecutionError,
      LLMProviderOutputError,
    ) as exc:
      self._mark_failure(
        metadata=running_metadata,
        artifact_root=prompt_result.artifacts.root_dir,
        exc=exc,
      )
      raise exc.__class__(self._sanitize(str(exc))) from exc
    else:
      final_metadata = self._artifact_planner.mark_stage_completed(
        running_metadata,
        stage=PipelineStage.REPORT,
      )
      self._metadata_repository.write(final_metadata)
      self._artifact_planner.append_log_event(
        prompt_result.artifacts.root_dir,
        "report generated "
        f"(provider={provider_name}, model={metadata.workflow.llm_model}, "
        f"chars={len(report_text)})",
      )
      final_metadata = self._complete_pdf_stage_if_needed(
        metadata=final_metadata,
        stage_decisions=prompt_result.stage_decisions,
        artifacts=prompt_result.artifacts,
      )
      return self._build_result(
        source=prompt_result.source,
        artifacts=prompt_result.artifacts,
        stage_decisions=prompt_result.stage_decisions,
        final_metadata=final_metadata,
        prompt_path=prompt_result.prompt_path,
        rendered_prompt=prompt_result.rendered_prompt,
        llm_response=llm_response,
        report_text=report_text,
      )

  def _complete_pdf_stage_if_needed(
    self,
    *,
    metadata: PipelineMetadata,
    stage_decisions: tuple[StageDecision, ...],
    artifacts,
  ) -> PipelineMetadata:
    pdf_decision = _stage_decision(stage_decisions, PipelineStage.PDF)
    if pdf_decision.decision != PipelineStageDecision.PLANNED:
      return metadata

    running_metadata = self._artifact_planner.mark_stage_running(
      metadata,
      stage=PipelineStage.PDF,
    )
    self._metadata_repository.write(running_metadata)
    self._artifact_planner.append_log_event(
      artifacts.root_dir,
      f"pdf rendering started (engine={self._renderer_engine_label()})",
    )

    try:
      self._document_renderer.render(
        artifacts.report_markdown,
        artifacts.report_pdf,
      )
    except (
      PDFRenderingConfigurationError,
      PDFRenderingExecutionError,
      PDFRenderingOutputError,
    ) as exc:
      self._mark_pdf_failure(
        metadata=running_metadata,
        artifact_root=artifacts.root_dir,
        exc=exc,
      )

    final_metadata = self._artifact_planner.mark_stage_completed(
      running_metadata,
      stage=PipelineStage.PDF,
    )
    self._metadata_repository.write(final_metadata)
    self._artifact_planner.append_log_event(
      artifacts.root_dir,
      f"pdf rendered (engine={self._renderer_engine_label()}, output={artifacts.report_pdf.name})",
    )
    return final_metadata

  def _mark_failure(
    self,
    *,
    metadata: PipelineMetadata,
    artifact_root: Path,
    exc: Exception,
  ) -> PipelineMetadata:
    sanitized_message = self._sanitize(str(exc))
    final_metadata = self._artifact_planner.mark_stage_failed(
      metadata,
      stage=PipelineStage.REPORT,
      error=StageErrorSummary(
        type=exc.__class__.__name__,
        code=_error_code(exc),
        message=sanitized_message,
      ),
    )
    self._metadata_repository.write(final_metadata)
    self._artifact_planner.append_log_event(
      artifact_root,
      f"report generation failed ({_error_code(exc)}): {sanitized_message}",
    )
    return final_metadata

  def _mark_pdf_failure(
    self,
    *,
    metadata: PipelineMetadata,
    artifact_root: Path,
    exc: PDFRenderingFailure,
  ) -> NoReturn:
    sanitized_message = self._sanitize(str(exc))
    final_metadata = self._artifact_planner.mark_stage_failed(
      metadata,
      stage=PipelineStage.PDF,
      error=StageErrorSummary(
        type=exc.__class__.__name__,
        code=_error_code(exc),
        message=sanitized_message,
      ),
    )
    self._metadata_repository.write(final_metadata)
    self._artifact_planner.append_log_event(
      artifact_root,
      f"pdf rendering failed ({_error_code(exc)}): {sanitized_message}",
    )
    raise _rebuild_pdf_exception(exc, sanitized_message) from exc

  def _sanitize(self, text: str) -> str:
    return redact_text(text, secrets=self._secret_values)

  def _build_result(
    self,
    *,
    source,
    artifacts,
    stage_decisions: tuple[StageDecision, ...],
    final_metadata: PipelineMetadata,
    prompt_path: Path,
    rendered_prompt: str,
    llm_response: str,
    report_text: str,
  ) -> GenerateReportResult:
    return GenerateReportResult(
      source=source,
      artifacts=artifacts,
      stage_decisions=stage_decisions,
      final_metadata=final_metadata,
      prompt_path=prompt_path,
      response_path=artifacts.llm_response_raw,
      report_path=artifacts.report_markdown,
      rendered_prompt=rendered_prompt,
      llm_response=llm_response,
      report_text=report_text,
      remote_provider_selected=final_metadata.workflow.llm_provider != "ollama",
    )

  def _renderer_engine_label(self) -> str:
    engine = getattr(self._document_renderer, "last_engine", None)
    return str(engine) if engine else "unknown"

  def _update_effective_workflow(
    self,
    metadata: PipelineMetadata,
    *,
    request: GenerateReportRequest,
  ) -> PipelineMetadata:
    return self._artifact_planner.update_workflow(
      metadata,
      template_name=request.template_name or metadata.workflow.template_name,
      llm_provider=request.llm_provider or metadata.workflow.llm_provider,
      llm_model=request.llm_model or metadata.workflow.llm_model,
      output_format=metadata.workflow.output_format,
      language=metadata.workflow.language,
      selected_stages=request.workflow_selected_stages,
    )


def _normalize_report_text(text: str) -> str:
  return text if text.endswith("\n") else f"{text}\n"


def _report_stage_decision(stage_decisions: tuple[StageDecision, ...]) -> StageDecision:
  return _stage_decision(stage_decisions, PipelineStage.REPORT)


def _stage_decision(
  stage_decisions: tuple[StageDecision, ...],
  stage: PipelineStage,
) -> StageDecision:
  for decision in stage_decisions:
    if decision.stage == stage:
      return decision
  raise PromptRenderPrerequisiteError(
    f"Report generation could not resolve the {stage.value} stage."
  )


def _error_code(exc: Exception) -> str:
  if isinstance(exc, LLMProviderConfigurationError):
    return "llm_provider_configuration_invalid"
  if isinstance(exc, LLMProviderExecutionError):
    return "llm_provider_execution_failed"
  if isinstance(exc, LLMProviderOutputError):
    return "llm_provider_output_invalid"
  if isinstance(exc, ReportArtifactPersistenceError):
    return "report_artifact_persistence_failed"
  if isinstance(exc, PDFRenderingConfigurationError):
    return "pdf_rendering_configuration_invalid"
  if isinstance(exc, PDFRenderingExecutionError):
    return "pdf_rendering_execution_failed"
  if isinstance(exc, PDFRenderingOutputError):
    return "pdf_rendering_output_invalid"
  return "report_generation_failed"


def _rebuild_pdf_exception(
  exc: PDFRenderingFailure,
  message: str,
) -> PDFRenderingFailure:
  if isinstance(exc, PDFRenderingExecutionError):
    return PDFRenderingExecutionError(
      engine=exc.engine,
      exit_code=exc.exit_code,
      stderr_summary=message,
    )
  if isinstance(exc, PDFRenderingConfigurationError):
    return PDFRenderingConfigurationError(message)
  return PDFRenderingOutputError(message)
