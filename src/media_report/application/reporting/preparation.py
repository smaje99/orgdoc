from __future__ import annotations

from pathlib import Path

from media_report.application.reporting.models import PreparedPromptRun, RenderPromptRequest
from media_report.core.errors import (
  ArtifactMetadataError,
  InputPathError,
  PromptRenderPrerequisiteError,
  StagePrerequisiteError,
)
from media_report.domain.artifacts.entities import (
  ArtifactPlan,
  PipelineMetadata,
  PipelineStage,
  PipelineStageDecision,
  PipelineStageStatus,
  StageDecision,
)
from media_report.domain.artifacts.ports import PipelineMetadataRepository
from media_report.domain.artifacts.service import (
  ArtifactPlanner,
  ArtifactRootValidator,
  PipelineStatePlanner,
)
from media_report.domain.media.entities import MediaKind, MediaSource
from media_report.infrastructure.filesystem.scanner import FileSystemMediaScanner


class PromptRunPreparer:
  def __init__(
    self,
    *,
    scanner: FileSystemMediaScanner,
    metadata_repository: PipelineMetadataRepository,
    artifact_planner: ArtifactPlanner,
    artifact_validator: ArtifactRootValidator,
    state_planner: PipelineStatePlanner,
  ) -> None:
    self._scanner = scanner
    self._metadata_repository = metadata_repository
    self._artifact_planner = artifact_planner
    self._artifact_validator = artifact_validator
    self._state_planner = state_planner

  def prepare(self, request: RenderPromptRequest) -> PreparedPromptRun:
    if not request.input_path.exists():
      raise InputPathError(f"Input path does not exist: {request.input_path}")
    if not request.input_path.is_dir():
      raise PromptRenderPrerequisiteError(
        "Prompt rendering requires an existing artifact directory, not a media file."
      )

    metadata_path = request.input_path / "metadata.json"
    if not metadata_path.exists():
      raise PromptRenderPrerequisiteError(
        f"Artifact metadata is missing for '{request.input_path.name}': {metadata_path}."
      )

    try:
      metadata = self._metadata_repository.read(metadata_path)
    except ArtifactMetadataError as exc:
      raise PromptRenderPrerequisiteError(str(exc)) from exc

    source = self._resolve_source(metadata)
    artifacts = self._artifact_planner.plan(source.path)
    if artifacts.root_dir != request.input_path:
      raise PromptRenderPrerequisiteError(
        "Artifact root does not match metadata source path: "
        f"expected '{artifacts.root_dir}', found '{request.input_path}'."
      )

    report_recovery_reason: str | None = None
    pdf_recovery_reason: str | None = None
    try:
      self._artifact_validator.validate(
        source=source,
        artifact_plan=artifacts,
        metadata=metadata,
      )
    except ArtifactMetadataError as exc:
      metadata, report_recovery_reason = self._recover_invalid_report_completion(
        metadata=metadata,
        artifacts=artifacts,
        error=exc,
      )
      metadata, pdf_recovery_reason = self._recover_invalid_pdf_completion(
        metadata=metadata,
        artifacts=artifacts,
        error=exc,
      )
      recovery_reason = report_recovery_reason or pdf_recovery_reason
      if recovery_reason is None:
        raise PromptRenderPrerequisiteError(str(exc)) from exc
    else:
      recovery_reason = None

    effective_template = request.template_name or metadata.workflow.template_name
    metadata = self._artifact_planner.update_workflow(
      metadata,
      template_name=effective_template,
      llm_provider=metadata.workflow.llm_provider,
      llm_model=metadata.workflow.llm_model,
      output_format=metadata.workflow.output_format,
      language=metadata.workflow.language,
      selected_stages=request.workflow_selected_stages,
    )

    if request.overwrite:
      metadata = self._artifact_planner.reset_stages_to_planned(
        metadata,
        stages=(PipelineStage.REPORT, PipelineStage.PDF),
      )

    try:
      stage_decisions = self._state_planner.plan_resume(
        metadata=metadata,
        requested_stages=request.workflow_selected_stages,
        force_stages={PipelineStage.REPORT} if request.overwrite else None,
      )
    except StagePrerequisiteError as exc:
      raise PromptRenderPrerequisiteError(str(exc)) from exc

    if recovery_reason is not None:
      stage_decisions = _replace_stage_decision_reason(
        stage_decisions=stage_decisions,
        stage=(PipelineStage.REPORT if report_recovery_reason is not None else PipelineStage.PDF),
        reason=recovery_reason,
      )

    self._metadata_repository.write(metadata)
    self._artifact_planner.ensure_log(artifacts.root_dir)
    if request.overwrite:
      self._artifact_planner.append_log_event(
        artifacts.root_dir,
        "report artifacts reset for regeneration; pdf stage marked planned.",
      )
    elif recovery_reason is not None:
      self._artifact_planner.append_log_event(
        artifacts.root_dir,
        recovery_reason,
      )
    return PreparedPromptRun(
      source=source,
      artifacts=artifacts,
      metadata=metadata,
      stage_decisions=stage_decisions,
    )

  def _resolve_source(self, metadata: PipelineMetadata) -> MediaSource:
    source_path = Path(metadata.source.path)
    if source_path.exists():
      return self._scanner.classify(source_path)
    return MediaSource(path=source_path, kind=MediaKind(metadata.source.kind))

  def _recover_invalid_report_completion(
    self,
    *,
    metadata: PipelineMetadata,
    artifacts: ArtifactPlan,
    error: ArtifactMetadataError,
  ) -> tuple[PipelineMetadata, str | None]:
    report_status = metadata.stages[PipelineStage.REPORT].status
    if report_status != PipelineStageStatus.COMPLETED:
      return metadata, None
    if "Stage 'report'" not in str(error):
      return metadata, None
    issue = self._artifact_validator.report_completion_issue(artifact_plan=artifacts)
    if issue is None:
      return metadata, None
    repaired_metadata = self._artifact_planner.reset_stages_to_planned(
      metadata,
      stages=(PipelineStage.REPORT, PipelineStage.PDF),
    )
    return (
      repaired_metadata,
      "Existing report artifacts were incomplete or inconsistent; rerunning report generation.",
    )

  def _recover_invalid_pdf_completion(
    self,
    *,
    metadata: PipelineMetadata,
    artifacts: ArtifactPlan,
    error: ArtifactMetadataError,
  ) -> tuple[PipelineMetadata, str | None]:
    pdf_status = metadata.stages[PipelineStage.PDF].status
    if pdf_status != PipelineStageStatus.COMPLETED:
      return metadata, None
    if "Stage 'pdf'" not in str(error):
      return metadata, None
    if self._artifact_validator.missing_outputs_for_stage(
      artifact_plan=artifacts,
      stage=PipelineStage.PDF,
    ):
      repaired_metadata = self._artifact_planner.reset_stages_to_planned(
        metadata,
        stages=(PipelineStage.PDF,),
      )
      return (
        repaired_metadata,
        "Existing pdf artifact was missing; rerunning pdf generation.",
      )
    return metadata, None


def _replace_stage_decision_reason(
  *,
  stage_decisions: tuple[StageDecision, ...],
  stage: PipelineStage,
  reason: str,
) -> tuple[StageDecision, ...]:
  return tuple(
    decision.model_copy(update={"reason": reason})
    if decision.stage == stage and decision.decision == PipelineStageDecision.PLANNED
    else decision
    for decision in stage_decisions
  )
