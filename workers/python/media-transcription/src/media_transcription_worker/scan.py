from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

from media_report.core.errors import InputPathError
from media_report.core.redaction import redact_text
from media_report.infrastructure.filesystem.scanner import FileSystemMediaScanner


def execute(request: dict[str, Any]) -> dict[str, Any]:
  job_id = str(request.get("job_id", ""))
  artifact_dir = Path(request.get("artifact_dir", ""))
  document = request.get("input", {}).get("document", {})
  try:
    if request.get("protocol_version") != 1 or not job_id:
      raise ValueError("protocol_version 1 and a non-empty job_id are required")
    if not artifact_dir.is_dir() or not isinstance(document, dict):
      raise ValueError("artifact_dir must exist and input.document must be an object")
    source_path = Path(document["source_path"]).expanduser().resolve(strict=True)
    scanner = FileSystemMediaScanner()
    if source_path.is_file():
      items = [scanner.classify(source_path)]
    elif source_path.is_dir():
      items = scanner.scan(source_path, recursive=bool(document.get("recursive", False)))
    else:
      raise InputPathError("source_path must identify a media file or directory")
    if not items:
      raise InputPathError("No supported audio or video files were found.")
    summary = {
      "schema_version": 1,
      "count": len(items),
      "items": [{"source_path": str(item.path), "kind": item.kind.value} for item in items],
    }
    (artifact_dir / "media-scan.json").write_text(
      json.dumps(summary, indent=2) + "\n", encoding="utf-8"
    )
    return _result(
      job_id, "succeeded", [{"path": "media-scan.json", "media_type": "application/json"}], []
    )
  except Exception as exc:
    return _result(
      job_id, "failed", [], [{"code": "media_scan_failed", "message": redact_text(str(exc))}]
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
