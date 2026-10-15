from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

from media_report.application.reporting import GenerateReportRequest
from media_report.cli.bootstrap import build_report_generation_service
from media_report.core.redaction import redact_text
from media_report.core.settings import AppSettings, load_settings
from media_report.domain.artifacts.entities import PipelineStage
from media_report.domain.artifacts.service import ArtifactPlanner


def execute(request: dict[str, Any]) -> dict[str, Any]:
  job_id = str(request.get("job_id", ""))
  artifact_dir = Path(request.get("artifact_dir", ""))
  document = request.get("input", {}).get("document", {})
  diagnostics: list[dict[str, str]] = []
  try:
    _validate_request(request, job_id, artifact_dir, document)
    settings = _settings(document)
    input_path = _artifact_path(document["artifact_path"])
    service = build_report_generation_service(settings)
    result = service.generate_report(
      GenerateReportRequest(
        input_path=input_path,
        template_name=document.get("template"),
        llm_provider=settings.llm_provider,
        llm_model=settings.llm_model,
        overwrite=bool(document.get("overwrite", False)),
        workflow_selected_stages=(PipelineStage.REPORT, PipelineStage.PDF)
        if document.get("include_pdf", True)
        else (PipelineStage.REPORT,),
      )
    )
    if result.remote_provider_selected:
      diagnostics.append(
        {
          "code": "remote_provider",
          "message": "Remote LLM provider selected; report content may leave this machine.",
        }
      )
    artifacts = result.artifacts
    summary = {
      "schema_version": 1,
      "legacy_metadata_schema_version": result.final_metadata.schema_version,
      "source_path": str(result.source.path),
      "artifact_root": str(artifacts.root_dir),
      "metadata_path": str(artifacts.metadata_json),
      "pipeline_log_path": str(artifacts.pipeline_log),
      "artifacts": {
        name: str(getattr(artifacts, name))
        for name in (
          "transcript_raw",
          "transcript_segments",
          "transcript_clean",
          "prompt_used",
          "llm_response_raw",
          "report_markdown",
          "report_pdf",
        )
      },
      "stage_decisions": [
        {"stage": item.stage.value, "decision": item.decision.value, "reason": item.reason}
        for item in result.stage_decisions
      ],
      "llm_provider": settings.llm_provider,
      "llm_model": settings.llm_model,
    }
    (artifact_dir / "media-result.json").write_text(
      json.dumps(summary, indent=2) + "\n", encoding="utf-8"
    )
  except Exception as exc:
    diagnostics.append({"code": "media_report_failed", "message": redact_text(str(exc))})
    return _result(job_id, "failed", [], diagnostics)
  return _result(
    job_id,
    "succeeded",
    [{"path": "media-result.json", "media_type": "application/json"}],
    diagnostics,
  )


def _settings(document: dict[str, Any]) -> AppSettings:
  settings = load_settings()
  overrides: dict[str, Any] = {}
  if document.get("provider"):
    overrides["llm_provider"] = document["provider"]
  if document.get("model"):
    overrides["llm_model"] = document["model"]
  if document.get("output_format"):
    overrides["output_format"] = document["output_format"]
  return settings.model_copy(update=overrides)


def _validate_request(
  request: dict[str, Any], job_id: str, artifact_dir: Path, document: Any
) -> None:
  if request.get("protocol_version") != 1 or not job_id:
    raise ValueError("protocol_version 1 and a non-empty job_id are required")
  if not artifact_dir.is_dir():
    raise ValueError("artifact_dir must be an existing directory")
  if not isinstance(document, dict):
    raise ValueError("input.document must be an object")


def _artifact_path(value: str) -> Path:
  input_path = Path(value).expanduser().resolve(strict=True)
  if input_path.is_file():
    input_path = ArtifactPlanner().plan(input_path).root_dir
  if not input_path.is_dir():
    raise ValueError("artifact_path must identify a media file or artifact directory")
  if not (input_path / "metadata.json").is_file():
    raise ValueError(f"artifact metadata is missing: {input_path / 'metadata.json'}")
  return input_path


def _result(
  job_id: str, status: str, outputs: list[dict[str, str]], diagnostics: list[dict[str, str]]
) -> dict[str, Any]:
  return {
    "protocol_version": 1,
    "job_id": job_id,
    "status": status,
    "outputs": outputs,
    "diagnostics": diagnostics,
  }


def main() -> int:
  try:
    request = json.load(sys.stdin)
    result = execute(request)
  except Exception as exc:
    result = _result(
      "", "failed", [], [{"code": "invalid_request", "message": redact_text(str(exc))}]
    )
  json.dump(result, sys.stdout, ensure_ascii=False)
  sys.stdout.write("\n")
  return 0 if result["status"] == "succeeded" else 1


if __name__ == "__main__":
  raise SystemExit(main())
