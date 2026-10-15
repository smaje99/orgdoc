from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

from media_report.application.process_media.models import ProcessRequest
from media_report.cli.bootstrap import build_process_service
from media_report.core.redaction import redact_text
from media_report.core.settings import AppSettings, load_settings


def execute(request: dict[str, Any]) -> dict[str, Any]:
  job_id = str(request.get("job_id", ""))
  artifact_dir = Path(request.get("artifact_dir", ""))
  document = request.get("input", {}).get("document", {})
  diagnostics: list[dict[str, str]] = []
  try:
    _extracted_from_execute_7(request, job_id, artifact_dir, document)
  except Exception as exc:
    diagnostics.append({"code": "media_transcription_failed", "message": redact_text(str(exc))})
    return _result(job_id, "failed", [], diagnostics)
  return _result(
    job_id,
    "succeeded",
    [{"path": "media-result.json", "media_type": "application/json"}],
    diagnostics,
  )


# TODO Rename this here and in `execute`
def _extracted_from_execute_7(request, job_id, artifact_dir, document):
  _validate_request(request, job_id, artifact_dir, document)
  settings = _settings(document)
  source_path = Path(document["source_path"]).expanduser().resolve(strict=True)
  if not source_path.is_file() and not source_path.is_dir():
    raise ValueError(
      "source_path must identify a media file, artifact directory, or scan directory"
    )
  service = build_process_service(settings)
  result = service.process(
    ProcessRequest(
      input_path=source_path,
      recursive=bool(document.get("recursive", False)),
      resume=bool(document.get("resume", False)),
      overwrite=bool(document.get("overwrite", False)),
      language=document.get("language"),
      transcription_device=settings.whisper_device,
      template_name=str(document.get("template", "generic")),
      llm_provider=settings.llm_provider,
      llm_model=settings.llm_model,
      output_format=settings.output_format,
      only_transcribe=True,
    )
  )
  summary = {
    "schema_version": 1,
    "runs": [_item_summary(item) for item in result.items],
  }
  (artifact_dir / "media-result.json").write_text(
    json.dumps(summary, indent=2) + "\n", encoding="utf-8"
  )


def _settings(document: dict[str, Any]) -> AppSettings:
  settings = load_settings()
  overrides: dict[str, Any] = {}
  if document.get("model"):
    overrides["whisper_model"] = document["model"]
  if document.get("device"):
    overrides["whisper_device"] = document["device"]
  if document.get("provider"):
    overrides["llm_provider"] = document["provider"]
  if document.get("llm_model"):
    overrides["llm_model"] = document["llm_model"]
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


def _item_summary(item: Any) -> dict[str, Any]:
  artifacts = item.artifacts
  metadata = item.final_metadata
  return {
    "legacy_metadata_schema_version": metadata.schema_version,
    "source_path": str(item.source.path),
    "artifact_root": str(artifacts.root_dir),
    "metadata_path": str(artifacts.metadata_json),
    "pipeline_log_path": str(artifacts.pipeline_log),
    "artifacts": {
      name: str(getattr(artifacts, name))
      for name in (
        "audio_extracted",
        "audio_normalized",
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
      {
        "stage": decision.stage.value,
        "decision": decision.decision.value,
        "reason": decision.reason,
      }
      for decision in item.stage_decisions
    ],
    "transcription": metadata.transcription.model_dump(mode="json")
    if metadata.transcription
    else None,
  }


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
