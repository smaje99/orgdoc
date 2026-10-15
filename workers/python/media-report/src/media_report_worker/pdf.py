from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

from media_report.application.reporting import GenerateReportRequest
from media_report.cli.bootstrap import build_report_generation_service
from media_report.core.redaction import redact_text
from media_report.core.settings import load_settings
from media_report.domain.artifacts.entities import PipelineStage
from media_report.domain.artifacts.service import ArtifactPlanner


def execute(request: dict[str, Any]) -> dict[str, Any]:
  job_id = str(request.get("job_id", ""))
  artifact_dir = Path(request.get("artifact_dir", ""))
  document = request.get("input", {}).get("document", {})
  try:
    return _extracted_from_execute_6(request, job_id, artifact_dir, document)
  except Exception as exc:
    return _result(
      job_id, "failed", [], [{"code": "media_pdf_failed", "message": redact_text(str(exc))}]
    )


# TODO Rename this here and in `execute`
def _extracted_from_execute_6(request, job_id, artifact_dir, document):
  if request.get("protocol_version") != 1 or not job_id:
    raise ValueError("protocol_version 1 and a non-empty job_id are required")
  if not artifact_dir.is_dir() or not isinstance(document, dict):
    raise ValueError("artifact_dir must exist and input.document must be an object")
  input_path = _artifact_path(document["artifact_path"])
  settings = load_settings()
  service = build_report_generation_service(settings)
  result = service.generate_report(
    GenerateReportRequest(
      input_path=input_path,
      llm_provider=settings.llm_provider,
      llm_model=settings.llm_model,
      workflow_selected_stages=(PipelineStage.PDF,),
    )
  )
  if not result.artifacts.report_pdf.is_file():
    raise FileNotFoundError("PDF worker completed without producing report.pdf")
  summary = {
    "schema_version": 1,
    "legacy_metadata_schema_version": result.final_metadata.schema_version,
    "source_path": str(result.source.path),
    "artifact_root": str(result.artifacts.root_dir),
    "metadata_path": str(result.artifacts.metadata_json),
    "pipeline_log_path": str(result.artifacts.pipeline_log),
    "report_markdown": str(result.artifacts.report_markdown),
    "report_pdf": str(result.artifacts.report_pdf),
  }
  (artifact_dir / "media-pdf.json").write_text(
    json.dumps(summary, indent=2) + "\n", encoding="utf-8"
  )
  return _result(
    job_id, "succeeded", [{"path": "media-pdf.json", "media_type": "application/json"}], []
  )


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


def _artifact_path(value: str) -> Path:
  input_path = Path(value).expanduser().resolve(strict=True)
  if input_path.is_file():
    input_path = ArtifactPlanner().plan(input_path).root_dir
  if not input_path.is_dir():
    raise ValueError("artifact_path must identify a media file or artifact directory")
  if not (input_path / "metadata.json").is_file():
    raise ValueError(f"artifact metadata is missing: {input_path / 'metadata.json'}")
  return input_path


def main() -> int:
  try:
    result = execute(json.load(sys.stdin))
  except Exception as exc:
    result = _result(
      "", "failed", [], [{"code": "invalid_request", "message": redact_text(str(exc))}]
    )
  json.dump(result, sys.stdout, ensure_ascii=False)
  sys.stdout.write("\n")
  return 0 if result["status"] == "succeeded" else 1


if __name__ == "__main__":
  raise SystemExit(main())
