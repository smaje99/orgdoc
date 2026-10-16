from __future__ import annotations

import hashlib
import json
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[4]
WORKER = ROOT / "workers" / "python" / "pdf-form" / "src" / "orgdoc_pdf_form" / "worker.py"


def generation_request(artifact_dir: Path) -> dict:
  return {
    "protocol_version": 1,
    "job_id": "generate-form-test",
    "operation_id": "consulting",
    "brand_id": "henko",
    "document_type": "pdf-form",
    "artifact_dir": str(artifact_dir),
    "input": {
      "document": {
        "title": "Contact request",
        "page_size": "A4",
        "fields": [
          {
            "name": "full_name",
            "type": "text",
            "label": "Full name",
            "x": 48,
            "y": 72,
            "width": 260,
            "height": 28,
            "required": True,
          },
          {
            "name": "consent",
            "type": "checkbox",
            "label": "I agree",
            "x": 48,
            "y": 122,
            "size": 16,
          },
          {
            "name": "contact_method",
            "type": "radio",
            "label": "Preferred contact",
            "x": 48,
            "y": 170,
            "size": 16,
            "gap": 110,
            "options": [
              {"value": "email", "label": "Email"},
              {"value": "phone", "label": "Phone"},
            ],
          },
          {
            "name": "country",
            "type": "choice",
            "label": "Country",
            "x": 48,
            "y": 230,
            "width": 220,
            "height": 28,
            "options": ["CO", "MX", "US"],
          },
        ],
      },
      "organization_root": str(artifact_dir),
      "context": {
        "organization": {"name": "Henko"},
        "operation": {"name": "Consulting"},
        "brand": {
          "id": "henko",
          "name": "Henko Technologies",
          "version": "1.0.0",
          "assets": [],
          "visual": {
            "colors": {
              "primary": "#173F5F",
              "secondary": "#20639B",
              "accent": "#F6D55C",
              "text": "#17212B",
            }
          },
        },
        "document_type": {"id": "pdf-form", "version": "1.0.0"},
      },
    },
  }


def run_worker(request: dict, operation: str) -> dict:
  completed = subprocess.run(
    [sys.executable, str(WORKER), operation],
    input=json.dumps(request),
    capture_output=True,
    text=True,
    check=False,
  )
  assert completed.returncode == 0, completed.stderr
  return json.loads(completed.stdout)


def create_template(directory: Path) -> Path:
  directory.mkdir(parents=True, exist_ok=True)
  result = run_worker(generation_request(directory), "generate")
  assert result["status"] == "succeeded", result["diagnostics"]
  return directory / "fillable-form.pdf"


def fill_request(organization: Path, template: Path, artifact_dir: Path) -> dict:
  return {
    "protocol_version": 1,
    "job_id": "fill-form-test",
    "operation_id": "consulting",
    "brand_id": "henko",
    "document_type": "pdf-form-fill",
    "artifact_dir": str(artifact_dir),
    "input": {
      "organization_root": str(organization),
      "document": {
        "template_path": str(template.relative_to(organization)),
        "field_values": {
          "full_name": "Ada Lovelace",
          "consent": True,
          "contact_method": "email",
          "country": "CO",
        },
      },
      "context": {
        "organization": {"name": "Henko"},
        "operation": {"name": "Consulting"},
        "brand": {"id": "henko", "name": "Henko Technologies", "version": "1.0.0"},
        "document_type": {"id": "pdf-form-fill", "version": "1.0.0"},
      },
    },
  }


def _widget_field_name(widget) -> str | None:
  parent = widget.get("/Parent")
  current = parent.get_object() if parent else widget
  name = current.get("/T")
  return str(name) if name is not None else None


def test_generation_cli_returns_editable_form_with_supported_field_types(tmp_path: Path) -> None:
  from pypdf import PdfReader

  result = run_worker(generation_request(tmp_path), "generate")

  assert result["status"] == "succeeded", result["diagnostics"]
  assert result["outputs"] == [{"path": "fillable-form.pdf", "media_type": "application/pdf"}]
  assert {item["name"] for item in result["field_summary"]} == {
    "full_name",
    "consent",
    "contact_method",
    "country",
  }
  fields = PdfReader(tmp_path / "fillable-form.pdf").get_fields()
  generated_reader = PdfReader(tmp_path / "fillable-form.pdf")
  assert generated_reader.metadata.title == "Contact request"
  assert generated_reader.metadata.author == "Henko Technologies"
  assert fields["full_name"]["/FT"] == "/Tx"
  assert fields["consent"]["/FT"] == "/Btn"
  assert fields["contact_method"]["/FT"] == "/Btn"
  assert fields["country"]["/FT"] == "/Ch"
  assert fields["country"]["/Opt"] == ["CO", "MX", "US"]
  assert fields["country"]["/V"] == ""


def test_fill_validates_types_preserves_source_and_writes_appearance_streams(
  tmp_path: Path,
) -> None:
  from pypdf import PdfReader

  organization = tmp_path / "organization"
  template = create_template(organization)
  source_hash = hashlib.sha256(template.read_bytes()).hexdigest()
  artifact_dir = tmp_path / "artifacts"
  request = fill_request(organization, template, artifact_dir)

  result = run_worker(request, "fill")

  assert result["status"] == "succeeded", result["diagnostics"]
  output = artifact_dir / "completed-form.pdf"
  fields = PdfReader(output).get_fields()
  assert fields["full_name"]["/V"] == "Ada Lovelace"
  assert str(fields["consent"]["/V"]) == "/Yes"
  assert str(fields["contact_method"]["/V"]) == "/email"
  assert fields["country"]["/V"] == "CO"
  assert hashlib.sha256(template.read_bytes()).hexdigest() == source_hash
  assert PdfReader(output).metadata.author == "Henko Technologies"
  assert all(item["populated"] for item in result["field_summary"])
  assert "Ada Lovelace" not in json.dumps(result)
  reader = PdfReader(output)
  widget_values = {
    "full_name": "Ada Lovelace",
    "consent": "/Yes",
    "contact_method": "/email",
    "country": "CO",
  }
  for page in reader.pages:
    for annotation in page.get("/Annots", []):
      widget = annotation.get_object()
      if widget.get("/Subtype") != "/Widget":
        continue
      assert widget.get("/AP", {}).get("/N") is not None
      parent = widget.get("/Parent")
      effective_value = parent.get_object().get("/V") if parent else widget.get("/V")
      widget_name = parent.get_object().get("/T") if parent else widget.get("/T")
      if widget_name in {"full_name", "consent", "contact_method", "country"}:
        assert str(effective_value) == widget_values[str(widget_name)]


def test_fill_rejects_unknown_fields_and_preserves_previous_output(tmp_path: Path) -> None:
  organization = tmp_path / "organization"
  template = create_template(organization)
  artifact_dir = tmp_path / "artifacts"
  artifact_dir.mkdir()
  previous = artifact_dir / "completed-form.pdf"
  previous.write_bytes(b"previous successful artifact")
  request = fill_request(organization, template, artifact_dir)
  request["input"]["document"]["field_values"]["unknown_field"] = "must not be ignored"

  result = run_worker(request, "fill")

  assert result["status"] == "failed"
  assert result["diagnostics"][0]["code"] == "unknown_form_field"
  assert previous.read_bytes() == b"previous successful artifact"


def test_fill_rejects_invalid_value_type_and_organization_path_escape(tmp_path: Path) -> None:
  organization = tmp_path / "organization"
  template = create_template(organization)
  artifact_dir = tmp_path / "artifacts"
  request = fill_request(organization, template, artifact_dir)
  request["input"]["document"]["field_values"]["consent"] = "yes"

  wrong_type = run_worker(request, "fill")
  assert wrong_type["status"] == "failed"
  assert wrong_type["diagnostics"][0]["code"] == "invalid_form_value"

  request["input"]["document"]["template_path"] = "../outside.pdf"
  escaped = run_worker(request, "fill")
  assert escaped["status"] == "failed"
  assert escaped["diagnostics"][0]["code"] == "invalid_template_path"


def test_fill_rejects_malformed_pdf_without_creating_output(tmp_path: Path) -> None:
  organization = tmp_path / "organization"
  organization.mkdir()
  malformed = organization / "malformed.pdf"
  malformed.write_bytes(b"not a PDF")
  artifact_dir = tmp_path / "artifacts"
  artifact_dir.mkdir()
  request = fill_request(organization, malformed, artifact_dir)
  request["input"]["document"]["template_path"] = malformed.name

  result = run_worker(request, "fill")

  assert result["status"] == "failed"
  assert result["diagnostics"][0]["code"] == "invalid_pdf_template"
  assert not (artifact_dir / "completed-form.pdf").exists()


def test_generation_rejects_unsupported_fields_without_replacing_success(tmp_path: Path) -> None:
  request = generation_request(tmp_path)
  result = run_worker(request, "generate")
  assert result["status"] == "succeeded", result["diagnostics"]
  output = tmp_path / "fillable-form.pdf"
  original = output.read_bytes()
  request["input"]["document"]["fields"][0]["type"] = "signature"

  rejected = run_worker(request, "generate")

  assert rejected["status"] == "failed"
  assert rejected["diagnostics"][0]["code"] == "unsupported_field_type"
  assert output.read_bytes() == original


def test_flatten_is_explicit_and_removes_form_fields_and_widgets(tmp_path: Path) -> None:
  from pypdf import PdfReader

  organization = tmp_path / "organization"
  template = create_template(organization)
  artifact_dir = tmp_path / "artifacts"
  request = fill_request(organization, template, artifact_dir)
  request["input"]["document"]["flatten"] = True

  result = run_worker(request, "fill")

  assert result["status"] == "succeeded", result["diagnostics"]
  reader = PdfReader(artifact_dir / "completed-form.pdf")
  assert reader.get_fields() is None
  assert "/AcroForm" not in reader.root_object
  assert not any(
    annotation.get_object().get("/Subtype") == "/Widget"
    for page in reader.pages
    for annotation in page.get("/Annots", [])
  )


def test_fill_reports_template_field_summary_without_values(tmp_path: Path) -> None:
  organization = tmp_path / "organization"
  template = create_template(organization)
  request = fill_request(organization, template, tmp_path / "artifacts")

  result = run_worker(request, "fill")

  assert result["status"] == "succeeded", result["diagnostics"]
  assert {item["name"] for item in result["field_summary"]} == {
    "full_name",
    "consent",
    "contact_method",
    "country",
  }
  assert all("value" not in item for item in result["field_summary"])


def test_field_summary_distinguishes_off_buttons_from_populated_values(tmp_path: Path) -> None:
  organization = tmp_path / "organization"
  template = create_template(organization)
  request = fill_request(organization, template, tmp_path / "artifacts")
  request["input"]["document"]["field_values"] = {"full_name": "Ada Lovelace"}

  result = run_worker(request, "fill")

  assert result["status"] == "succeeded", result["diagnostics"]
  summary = {item["name"]: item["populated"] for item in result["field_summary"]}
  assert summary == {
    "full_name": True,
    "consent": False,
    "contact_method": False,
    "country": False,
  }
