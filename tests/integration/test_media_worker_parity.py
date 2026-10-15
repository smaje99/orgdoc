from __future__ import annotations

import importlib
import json
import os
import subprocess
import sys
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock

WORKER_ROOT = Path(__file__).resolve().parents[2] / "workers" / "python"
REPO_ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(WORKER_ROOT / "media-transcription" / "src"))
sys.path.insert(0, str(WORKER_ROOT / "media-report" / "src"))


def test_transcription_worker_preserves_legacy_artifact_references(tmp_path: Path) -> None:
  module = importlib.import_module("media_transcription_worker.main")
  artifact_root = tmp_path / "meeting_media_report"
  service = Mock()
  item = SimpleNamespace(
    source=SimpleNamespace(path=tmp_path / "meeting.mp4"),
    artifacts=SimpleNamespace(
      root_dir=artifact_root,
      metadata_json=artifact_root / "metadata.json",
      pipeline_log=artifact_root / "pipeline.log",
      audio_extracted=artifact_root / "audio_extracted.wav",
      audio_normalized=artifact_root / "audio_normalized.wav",
      transcript_raw=artifact_root / "transcript_raw.txt",
      transcript_segments=artifact_root / "transcript_segments.json",
      transcript_clean=artifact_root / "transcript_clean.md",
      prompt_used=artifact_root / "prompt_used.md",
      llm_response_raw=artifact_root / "llm_response_raw.txt",
      report_markdown=artifact_root / "report.md",
      report_pdf=artifact_root / "report.pdf",
    ),
    final_metadata=SimpleNamespace(schema_version=2, transcription=None),
    stage_decisions=(),
  )
  service = Mock()
  service.process.return_value = SimpleNamespace(items=[item])
  module.build_process_service = Mock(return_value=service)
  module.load_settings = Mock(return_value=module.AppSettings())
  source_path = tmp_path / "meeting.mp4"
  source_path.touch()
  artifact_dir = tmp_path / "worker-output"
  artifact_dir.mkdir()

  result = module.execute(
    {
      "protocol_version": 1,
      "job_id": "transcribe-1",
      "artifact_dir": str(artifact_dir),
      "input": {
        "document": {
          "source_path": str(source_path),
          "language": "es",
          "resume": True,
        }
      },
    }
  )

  assert result["status"] == "succeeded", result
  assert result["job_id"] == "transcribe-1"
  assert result["outputs"] == [{"path": "media-result.json", "media_type": "application/json"}]
  assert result["diagnostics"] == []
  service.process.assert_called_once()
  process_request = service.process.call_args.args[0]
  assert process_request.input_path == source_path.resolve()
  assert process_request.resume is True
  assert process_request.only_transcribe is True
  summary = json.loads((artifact_dir / "media-result.json").read_text(encoding="utf-8"))
  assert summary["runs"][0]["legacy_metadata_schema_version"] == 2
  assert summary["runs"][0]["artifact_root"] == str(artifact_root)
  assert summary["runs"][0]["pipeline_log_path"] == str(artifact_root / "pipeline.log")
  assert summary["runs"][0]["artifacts"]["transcript_raw"] == str(
    artifact_root / "transcript_raw.txt"
  )
  assert summary["runs"][0]["artifacts"]["llm_response_raw"] == str(
    artifact_root / "llm_response_raw.txt"
  )


def test_media_scan_worker_only_finds_supported_files_and_can_recurse(tmp_path: Path) -> None:
  module = importlib.import_module("media_transcription_worker.scan")
  source_dir = tmp_path / "media"
  nested = source_dir / "nested"
  nested.mkdir(parents=True)
  (source_dir / "first.mp3").touch()
  (source_dir / "ignore.txt").touch()
  (nested / "second.mp4").touch()
  artifact_dir = tmp_path / "worker-output"
  artifact_dir.mkdir()

  result = module.execute(
    {
      "protocol_version": 1,
      "job_id": "scan-1",
      "artifact_dir": str(artifact_dir),
      "input": {"document": {"source_path": str(source_dir), "recursive": True}},
    }
  )

  assert result["status"] == "succeeded"
  scan = json.loads((artifact_dir / "media-scan.json").read_text(encoding="utf-8"))
  assert scan["count"] == 2
  assert [item["kind"] for item in scan["items"]] == ["audio", "video"]


def test_media_scan_runs_through_orgdoc_capability_contract(tmp_path: Path) -> None:
  binary = tmp_path / "orgdoc"
  subprocess.run(["go", "build", "-o", str(binary), "./cmd/orgdoc"], cwd=REPO_ROOT, check=True)
  source = tmp_path / "meeting.mp3"
  source.touch()
  request = tmp_path / "scan-input.json"
  request.write_text(json.dumps({"source_path": str(source)}), encoding="utf-8")
  env = os.environ.copy()
  worker_bin = WORKER_ROOT / "media-transcription" / ".venv" / "bin"
  env["PATH"] = os.pathsep.join([str(worker_bin), env.get("PATH", "")])

  completed = subprocess.run(
    [
      str(binary),
      "--format",
      "json",
      "document",
      "render",
      "--organization",
      str(REPO_ROOT / "examples" / "organization-docs"),
      "--operation",
      "media",
      "--document-type",
      "media-scan",
      "--input",
      str(request),
      "--output-root",
      str(tmp_path / "runs"),
      "--job-id",
      "media-scan-1",
    ],
    cwd=REPO_ROOT,
    env=env,
    check=True,
    capture_output=True,
    text=True,
  )
  result = json.loads(completed.stdout)
  assert result["status"] == "succeeded"
  scan_path = Path(result["artifact_dir"]) / "media-scan.json"
  scan = json.loads(scan_path.read_text(encoding="utf-8"))
  assert scan["items"] == [{"source_path": str(source), "kind": "audio"}]


def test_report_worker_warns_for_remote_provider_without_exposing_secrets(tmp_path: Path) -> None:
  module = importlib.import_module("media_report_worker.main")
  input_path = tmp_path / "meeting_media_report"
  input_path.mkdir()
  (input_path / "metadata.json").write_text("{}", encoding="utf-8")
  artifact_dir = tmp_path / "worker-output"
  artifact_dir.mkdir()
  generated = SimpleNamespace(
    source=SimpleNamespace(path=tmp_path / "meeting.mp4"),
    artifacts=SimpleNamespace(
      root_dir=tmp_path / "meeting_media_report",
      metadata_json=tmp_path / "meeting_media_report" / "metadata.json",
      pipeline_log=tmp_path / "meeting_media_report" / "pipeline.log",
      transcript_raw=tmp_path / "meeting_media_report" / "transcript_raw.txt",
      transcript_segments=tmp_path / "meeting_media_report" / "transcript_segments.json",
      transcript_clean=tmp_path / "meeting_media_report" / "transcript_clean.md",
      prompt_used=tmp_path / "meeting_media_report" / "prompt_used.md",
      llm_response_raw=tmp_path / "meeting_media_report" / "llm_response_raw.txt",
      report_markdown=tmp_path / "meeting_media_report" / "report.md",
      report_pdf=tmp_path / "meeting_media_report" / "report.pdf",
    ),
    final_metadata=SimpleNamespace(
      schema_version=2, workflow=SimpleNamespace(llm_provider="openai")
    ),
    remote_provider_selected=True,
    stage_decisions=(),
    report_path=tmp_path / "meeting_media_report" / "report.md",
    response_path=tmp_path / "meeting_media_report" / "llm_response_raw.txt",
    prompt_path=tmp_path / "meeting_media_report" / "prompt_used.md",
  )
  service = Mock()
  service.generate_report.return_value = generated
  module.build_report_generation_service = Mock(return_value=service)
  module.load_settings = Mock(return_value=module.AppSettings())

  result = module.execute(
    {
      "protocol_version": 1,
      "job_id": "report-1",
      "artifact_dir": str(artifact_dir),
      "input": {"document": {"artifact_path": str(input_path), "provider": "openai"}},
    }
  )

  assert result["status"] == "succeeded", result
  assert result["job_id"] == "report-1"
  assert any(item["code"] == "remote_provider" for item in result["diagnostics"])
  summary = json.loads((artifact_dir / "media-result.json").read_text(encoding="utf-8"))
  assert summary["legacy_metadata_schema_version"] == 2
  assert summary["artifact_root"] == str(generated.artifacts.root_dir)
  assert summary["pipeline_log_path"] == str(generated.artifacts.pipeline_log)
  assert summary["artifacts"]["transcript_raw"] == str(generated.artifacts.transcript_raw)
  assert summary["artifacts"]["llm_response_raw"] == str(generated.artifacts.llm_response_raw)
  assert "api_key" not in json.dumps(summary)
  assert service.generate_report.call_args.args[0].input_path == input_path
  service.generate_report.assert_called_once()


def test_pdf_worker_only_runs_the_pdf_stage(tmp_path: Path) -> None:
  module = importlib.import_module("media_report_worker.pdf")
  source_artifact = tmp_path / "meeting_media_report"
  source_artifact.mkdir()
  (source_artifact / "metadata.json").write_text("{}", encoding="utf-8")
  input_path = tmp_path / "source"
  input_path.write_text("source", encoding="utf-8")
  report_pdf = source_artifact / "report.pdf"
  report_pdf.touch()
  service = Mock()
  service.generate_report.return_value = SimpleNamespace(
    source=SimpleNamespace(path=tmp_path / "meeting.mp4"),
    final_metadata=SimpleNamespace(schema_version=2),
    artifacts=SimpleNamespace(
      report_pdf=report_pdf,
      report_markdown=source_artifact / "report.md",
      root_dir=source_artifact,
      metadata_json=source_artifact / "metadata.json",
      pipeline_log=source_artifact / "pipeline.log",
    ),
  )
  module.load_settings = Mock(return_value=module.load_settings())
  module.build_report_generation_service = Mock(return_value=service)
  artifact_dir = tmp_path / "worker-output"
  artifact_dir.mkdir()

  result = module.execute(
    {
      "protocol_version": 1,
      "job_id": "pdf-1",
      "artifact_dir": str(artifact_dir),
      "input": {"document": {"artifact_path": str(source_artifact)}},
    }
  )

  assert result["status"] == "succeeded", result
  service.generate_report.assert_called_once()
  assert service.generate_report.call_args.args[0].workflow_selected_stages == (
    module.PipelineStage.PDF,
  )
