from __future__ import annotations

import json
import math
import os
import re
import sys
import tempfile
from pathlib import Path
from typing import Any

from pypdf import PdfReader, PdfWriter
from pypdf.generic import ArrayObject, DecodedStreamObject, DictionaryObject, NameObject
from reportlab.lib import colors
from reportlab.lib.pagesizes import A4, LETTER
from reportlab.pdfgen import canvas

PROTOCOL_VERSION = 1
MAX_FIELDS = 100
MAX_TEMPLATE_BYTES = 50 * 1024 * 1024
MAX_TEMPLATE_PAGES = 200
MAX_TEMPLATE_FIELDS = 500
MAX_TEMPLATE_WIDGETS = 1000
FIELD_NAME = re.compile(r"^[A-Za-z][A-Za-z0-9_.-]{0,63}$")
HEX_COLOR = re.compile(r"^#?[0-9A-Fa-f]{6}$")
PAGE_SIZES = {"A4": A4, "Letter": LETTER}
FIELD_TYPES = {"text": "/Tx", "checkbox": "/Btn", "radio": "/Btn", "choice": "/Ch"}


class WorkerError(ValueError):
  def __init__(self, code: str, message: str):
    super().__init__(message)
    self.code = code


def _failed(request: dict[str, Any], code: str, message: str) -> dict[str, Any]:
  return {
    "protocol_version": PROTOCOL_VERSION,
    "job_id": request.get("job_id", ""),
    "status": "failed",
    "outputs": [],
    "field_summary": [],
    "diagnostics": [{"code": code, "message": message}],
  }


def _success(
  request: dict[str, Any], filename: str, summary: list[dict[str, Any]]
) -> dict[str, Any]:
  return {
    "protocol_version": PROTOCOL_VERSION,
    "job_id": request.get("job_id", ""),
    "status": "succeeded",
    "outputs": [{"path": filename, "media_type": "application/pdf"}],
    "field_summary": summary,
    "diagnostics": [],
  }


def _request_context(
  request: dict[str, Any], expected_type: str
) -> tuple[dict[str, Any], dict[str, Any]]:
  if request.get("protocol_version") != PROTOCOL_VERSION:
    raise WorkerError("unsupported_protocol_version", "protocol_version must be 1")
  if request.get("document_type") != expected_type:
    raise WorkerError("invalid_request", f"document_type must be {expected_type!r}")
  payload = request.get("input")
  if not isinstance(payload, dict):
    raise WorkerError("invalid_request", "input must be an object")
  document = payload.get("document")
  context = payload.get("context")
  if not isinstance(document, dict) or not isinstance(context, dict):
    raise WorkerError("invalid_request", "input must contain a document and context objects")
  if not isinstance(context.get("brand"), dict):
    raise WorkerError("invalid_request", "input.context.brand must be an object")
  return document, context


def _text(value: Any, field: str, maximum: int = 500) -> str:
  if not isinstance(value, str) or not value.strip():
    raise WorkerError("invalid_form_definition", f"{field} must be non-empty text")
  if len(value) > maximum or "\x00" in value:
    raise WorkerError("invalid_form_definition", f"{field} exceeds the supported text limit")
  return value


def _number(value: Any, field: str, *, minimum: float = 0) -> float:
  if isinstance(value, bool) or not isinstance(value, (int, float)) or not math.isfinite(value):
    raise WorkerError("invalid_form_definition", f"{field} must be a finite number")
  if value < minimum:
    raise WorkerError("invalid_form_definition", f"{field} must be at least {minimum}")
  return float(value)


def _validate_generate(
  document: dict[str, Any],
) -> tuple[str, tuple[float, float], list[dict[str, Any]]]:
  title = _text(document.get("title"), "document.title", 255)
  page_size_name = document.get("page_size", "A4")
  if page_size_name not in PAGE_SIZES:
    raise WorkerError("invalid_form_definition", "page_size must be A4 or Letter")
  page_size = PAGE_SIZES[page_size_name]
  fields = document.get("fields")
  if not isinstance(fields, list) or not 1 <= len(fields) <= MAX_FIELDS:
    raise WorkerError("invalid_form_definition", f"fields must contain 1-{MAX_FIELDS} definitions")
  seen: set[str] = set()
  normalized: list[dict[str, Any]] = []
  page_width, page_height = page_size
  for index, item in enumerate(fields):
    if not isinstance(item, dict):
      raise WorkerError("invalid_form_definition", f"fields[{index}] must be an object")
    kind = item.get("type")
    if kind not in FIELD_TYPES:
      raise WorkerError("unsupported_field_type", f"fields[{index}] uses an unsupported type")
    name = item.get("name")
    if not isinstance(name, str) or not FIELD_NAME.fullmatch(name) or name in seen:
      raise WorkerError("invalid_form_definition", f"fields[{index}].name is invalid or duplicated")
    seen.add(name)
    field = dict(item)
    field["name"] = name
    field["label"] = _text(item.get("label"), f"fields[{index}].label", 160)
    field["x"] = _number(item.get("x"), f"fields[{index}].x")
    field["y"] = _number(item.get("y"), f"fields[{index}].y")
    field["required"] = item.get("required", False)
    if not isinstance(field["required"], bool):
      raise WorkerError("invalid_form_definition", f"fields[{index}].required must be boolean")

    if kind in {"text", "choice"}:
      field["width"] = _number(item.get("width"), f"fields[{index}].width", minimum=20)
      field["height"] = _number(item.get("height"), f"fields[{index}].height", minimum=16)
      if (
        field["x"] + field["width"] > page_width or field["y"] + field["height"] + 22 > page_height
      ):
        raise WorkerError(
          "invalid_form_definition", f"fields[{index}] does not fit on the selected page"
        )
      if kind == "text":
        field["max_length"] = item.get("max_length", 200)
        field["multiline"] = item.get("multiline", False)
        if (
          isinstance(field["max_length"], bool)
          or not isinstance(field["max_length"], int)
          or not 1 <= field["max_length"] <= 4000
        ):
          raise WorkerError(
            "invalid_form_definition", f"fields[{index}].max_length must be between 1 and 4000"
          )
        if not isinstance(field["multiline"], bool):
          raise WorkerError("invalid_form_definition", f"fields[{index}].multiline must be boolean")
        value = item.get("value", "")
        if not isinstance(value, str) or len(value) > field["max_length"] or "\x00" in value:
          raise WorkerError("invalid_form_definition", f"fields[{index}].value is not valid text")
        field["value"] = value
      else:
        options = item.get("options")
        if not isinstance(options, list) or not 1 <= len(options) <= 100:
          raise WorkerError(
            "invalid_form_definition", f"fields[{index}].options must contain 1-100 choices"
          )
        if any(
          not isinstance(option, str) or not option.strip() or len(option) > 120
          for option in options
        ):
          raise WorkerError(
            "invalid_form_definition", f"fields[{index}].options must be short non-empty strings"
          )
        if len(set(options)) != len(options):
          raise WorkerError("invalid_form_definition", f"fields[{index}].options must be unique")
        field["options"] = options
        value = item.get("value", "")
        if value and value not in options:
          raise WorkerError(
            "invalid_form_definition", f"fields[{index}].value must be one of its choices"
          )
        field["value"] = value
    elif kind == "checkbox":
      field["size"] = _number(item.get("size"), f"fields[{index}].size", minimum=10)
      field["value"] = item.get("value", False)
      if not isinstance(field["value"], bool):
        raise WorkerError("invalid_form_definition", f"fields[{index}].value must be boolean")
      if (
        field["x"] + field["size"] + len(field["label"]) * 7 + 16 > page_width
        or field["y"] + field["size"] > page_height
      ):
        raise WorkerError(
          "invalid_form_definition", f"fields[{index}] does not fit on the selected page"
        )
    else:
      options = item.get("options")
      if not isinstance(options, list) or not 2 <= len(options) <= 8:
        raise WorkerError(
          "invalid_form_definition", f"fields[{index}].options must contain 2-8 radio choices"
        )
      if any(not isinstance(option, dict) for option in options):
        raise WorkerError("invalid_form_definition", f"fields[{index}].options must be objects")
      for option in options:
        _text(option.get("value"), f"fields[{index}].option.value", 64)
        _text(option.get("label"), f"fields[{index}].option.label", 80)
      values = [option["value"] for option in options]
      if len(set(values)) != len(values):
        raise WorkerError("invalid_form_definition", f"fields[{index}].radio values must be unique")
      size = _number(item.get("size", 14), f"fields[{index}].size", minimum=10)
      gap = _number(item.get("gap", 105), f"fields[{index}].gap", minimum=size + 12)
      field["size"], field["gap"], field["options"] = size, gap, options
      selected = item.get("value", "")
      if selected and selected not in values:
        raise WorkerError(
          "invalid_form_definition", f"fields[{index}].value must be one of its radio values"
        )
      field["value"] = selected
      if (
        field["x"] + (len(options) - 1) * gap + size + 90 > page_width
        or field["y"] + size > page_height
      ):
        raise WorkerError(
          "invalid_form_definition", f"fields[{index}] does not fit on the selected page"
        )
    normalized.append(field)
  return title, page_size, normalized


def _palette(context: dict[str, Any]) -> tuple[Any, Any, Any]:
  visual = context["brand"].get("visual", {})
  colors_profile = visual.get("colors", {}) if isinstance(visual, dict) else {}
  if not isinstance(colors_profile, dict):
    colors_profile = {}

  def parse(key: str, fallback: str):
    raw = colors_profile.get(key)
    if isinstance(raw, str) and HEX_COLOR.fullmatch(raw):
      return colors.HexColor(raw)
    return colors.HexColor(fallback)

  return parse("primary", "#173F5F"), parse("text", "#17212B"), parse("accent", "#F6D55C")


def _top_y(page_height: float, top: float, height: float = 0) -> float:
  return page_height - top - height


def _draw_form_field(
  pdf: Any, field: dict[str, Any], page_height: float, primary: Any, text_color: Any
) -> None:
  form = pdf.acroForm
  kind = field["type"]
  x, top = field["x"], field["y"]
  required = field["required"]
  flags = "required" if required else ""
  if kind in {"text", "choice"}:
    pdf.setFillColor(text_color)
    pdf.setFont("Helvetica", 9)
    pdf.drawString(x, _top_y(page_height, top, 10), field["label"])
    y = _top_y(page_height, top + 22, field["height"])
    if kind == "text":
      text_flags = " ".join(
        token for token in (flags, "multiline" if field["multiline"] else "") if token
      )
      form.textfield(
        name=field["name"],
        tooltip=field["label"],
        x=x,
        y=y,
        width=field["width"],
        height=field["height"],
        value=field["value"],
        maxlen=field["max_length"],
        fontName="Helvetica",
        fontSize=10,
        borderStyle="solid",
        borderWidth=1,
        borderColor=primary,
        fillColor=colors.white,
        textColor=text_color,
        forceBorder=True,
        fieldFlags=text_flags,
      )
    else:
      choice_flags = " ".join(token for token in (flags, "combo") if token)
      form.choice(
        name=field["name"],
        tooltip=field["label"],
        options=field["options"],
        value=field["value"] or field["options"][0],
        x=x,
        y=y,
        width=field["width"],
        height=field["height"],
        fontName="Helvetica",
        fontSize=10,
        borderStyle="solid",
        borderWidth=1,
        borderColor=primary,
        fillColor=colors.white,
        textColor=text_color,
        forceBorder=True,
        fieldFlags=choice_flags,
      )
  elif kind == "checkbox":
    size = field["size"]
    y = _top_y(page_height, top, size)
    form.checkbox(
      name=field["name"],
      tooltip=field["label"],
      x=x,
      y=y,
      size=size,
      checked=field["value"],
      buttonStyle="check",
      borderWidth=1,
      borderColor=primary,
      fillColor=colors.white,
      textColor=primary,
      forceBorder=True,
      fieldFlags=flags,
    )
    pdf.setFillColor(text_color)
    pdf.setFont("Helvetica", 9)
    pdf.drawString(x + size + 7, y + 3, field["label"])
  else:
    radio_flags = " ".join(
      token for token in ("radio", "noToggleToOff" if required else "") if token
    )
    if required:
      radio_flags += " required"
    for index, option in enumerate(field["options"]):
      option_x = x + index * field["gap"]
      y = _top_y(page_height, top, field["size"])
      form.radio(
        name=field["name"],
        value=option["value"],
        selected=field["value"] == option["value"],
        x=option_x,
        y=y,
        size=field["size"],
        tooltip=option["label"],
        buttonStyle="circle",
        shape="circle",
        borderWidth=1,
        borderColor=primary,
        fillColor=colors.white,
        textColor=primary,
        forceBorder=True,
        fieldFlags=radio_flags,
      )
      pdf.setFillColor(text_color)
      pdf.setFont("Helvetica", 9)
      pdf.drawString(option_x + field["size"] + 6, y + 3, option["label"])


def _artifact_path(request: dict[str, Any], filename: str) -> tuple[Path, Path]:
  raw = request.get("artifact_dir")
  if not isinstance(raw, str) or not raw:
    raise WorkerError("invalid_request", "artifact_dir must be a non-empty path")
  artifact_dir = Path(raw)
  artifact_dir.mkdir(parents=True, exist_ok=True)
  output = artifact_dir / filename
  temporary_dir = Path(tempfile.mkdtemp(prefix=".orgdoc-pdf-form-", dir=artifact_dir))
  return output, temporary_dir / filename


def _verify_generated(path: Path, fields: list[dict[str, Any]]) -> None:
  reader = PdfReader(path, strict=True)
  actual = reader.get_fields() or {}
  missing = {field["name"] for field in fields} - set(actual)
  if missing:
    raise WorkerError(
      "form_generation_failed", f"generated PDF is missing fields: {sorted(missing)}"
    )
  for page in reader.pages:
    for annotation in page.get("/Annots", []):
      widget = annotation.get_object()
      if widget.get("/Subtype") == "/Widget" and not widget.get("/AP", {}).get("/N"):
        raise WorkerError(
          "form_generation_failed", "generated PDF contains a widget without an appearance"
        )


def generate(request: dict[str, Any]) -> dict[str, Any]:
  temporary_dir: Path | None = None
  try:
    document, context = _request_context(request, "pdf-form")
    title, page_size, fields = _validate_generate(document)
    primary, text_color, accent = _palette(context)
    output, staged = _artifact_path(request, "fillable-form.pdf")
    temporary_dir = staged.parent
    pdf = canvas.Canvas(str(staged), pagesize=page_size, pageCompression=1)
    pdf.setTitle(title)
    pdf.setAuthor(str(context["brand"].get("name") or "OrgDoc"))
    pdf.setSubject("Interactive AcroForm generated by OrgDoc")
    page_width, page_height = page_size
    pdf.setFillColor(primary)
    pdf.setFont("Helvetica-Bold", 16)
    pdf.drawString(48, page_height - 44, title)
    pdf.setStrokeColor(accent)
    pdf.setLineWidth(2)
    pdf.line(48, page_height - 55, page_width - 48, page_height - 55)
    for field in fields:
      _draw_form_field(pdf, field, page_height, primary, text_color)
    pdf.showPage()
    pdf.save()
    empty_choices = {
      field["name"]: "" for field in fields if field["type"] == "choice" and not field["value"]
    }
    if empty_choices:
      reader = PdfReader(staged, strict=True)
      writer = PdfWriter(clone_from=reader)
      writer.update_page_form_field_values(None, empty_choices, auto_regenerate=False)
      blanked = temporary_dir / "blanked-form.pdf"
      with blanked.open("wb") as stream:
        writer.write(stream)
      os.replace(blanked, staged)
    _verify_generated(staged, fields)
    os.replace(staged, output)
    summary = [
      {"name": field["name"], "type": field["type"], "populated": bool(field["value"])}
      for field in fields
    ]
    return _success(request, output.name, summary)
  except WorkerError as exc:
    return _failed(request, exc.code, str(exc))
  except Exception as exc:
    return _failed(
      request, "form_generation_failed", f"PDF form generation failed ({type(exc).__name__})"
    )
  finally:
    if temporary_dir is not None:
      import shutil

      shutil.rmtree(temporary_dir, ignore_errors=True)


def _resolve_template(payload: dict[str, Any]) -> Path:
  root_value, relative = payload.get("organization_root"), payload.get("template_path")
  if (
    not isinstance(root_value, str)
    or not root_value
    or not isinstance(relative, str)
    or not relative
  ):
    raise WorkerError("invalid_template_path", "organization_root and template_path are required")
  relative_path = Path(relative)
  if relative_path.is_absolute():
    raise WorkerError("invalid_template_path", "template_path must be relative to the organization")
  root = Path(root_value).resolve()
  try:
    template = (root / relative_path).resolve(strict=True)
  except OSError as exc:
    raise WorkerError(
      "invalid_template_path", "template_path does not exist inside the organization"
    ) from exc
  if (
    not template.is_relative_to(root) or not template.is_file() or template.suffix.lower() != ".pdf"
  ):
    raise WorkerError(
      "invalid_template_path", "template_path must be a PDF file inside the organization"
    )
  if template.stat().st_size > MAX_TEMPLATE_BYTES:
    raise WorkerError("invalid_pdf_template", "template PDF exceeds the 50 MiB size limit")
  return template


def _qualified_name(field: Any) -> str | None:
  name = field.get("/T")
  parent_ref = field.get("/Parent")
  parent = parent_ref.get_object() if parent_ref else None
  parent_name = _qualified_name(parent) if parent is not None else None
  if name is None:
    return parent_name
  return f"{parent_name}.{name}" if parent_name else str(name)


def _widgets(reader: PdfReader) -> list[Any]:
  found = []
  for page in reader.pages:
    for annotation_ref in page.get("/Annots", []):
      annotation = annotation_ref.get_object()
      if annotation.get("/Subtype") == "/Widget":
        found.append(annotation)
  return found


def _same_object(left: Any, right: Any) -> bool:
  left_ref, right_ref = (
    getattr(left, "indirect_reference", None),
    getattr(right, "indirect_reference", None),
  )
  return left is right or (left_ref is not None and left_ref == right_ref)


def _load_form(template: Path) -> tuple[PdfReader, dict[str, Any], list[Any]]:
  try:
    reader = PdfReader(template, strict=True)
    if reader.is_encrypted:
      raise WorkerError("unsupported_pdf_template", "encrypted PDF templates are not supported")
    if len(reader.pages) > MAX_TEMPLATE_PAGES:
      raise WorkerError(
        "invalid_pdf_template", f"template exceeds the {MAX_TEMPLATE_PAGES} page limit"
      )
    root = reader.trailer["/Root"]
    acro_form = root.get("/AcroForm")
    if acro_form:
      acro_form = acro_form.get_object()
      if acro_form.get("/XFA"):
        raise WorkerError(
          "unsupported_pdf_template", "XFA forms are not supported; use an AcroForm PDF"
        )
    fields = reader.get_fields() or {}
    widgets = _widgets(reader)
    if len(fields) > MAX_TEMPLATE_FIELDS or len(widgets) > MAX_TEMPLATE_WIDGETS:
      raise WorkerError("invalid_pdf_template", "template exceeds the form field or widget limit")
  except WorkerError:
    raise
  except Exception as exc:
    raise WorkerError("invalid_pdf_template", "could not parse the PDF form template") from exc
  if not widgets:
    raise WorkerError("invalid_pdf_template", "template PDF has no AcroForm widget annotations")

  orphans: dict[str, list[Any]] = {}
  for widget in widgets:
    name = _qualified_name(widget)
    parent = widget.get("/Parent")
    field = fields.get(name) if name else None
    if field is not None and not parent and not _same_object(widget, field):
      raise WorkerError(
        "ambiguous_pdf_form",
        f"widget and canonical field {name!r} are distinct objects with no parent relationship",
      )
    if field is None:
      if not name:
        raise WorkerError("ambiguous_pdf_form", "an orphan widget has no field name")
      orphans.setdefault(name, []).append(widget)
  if orphans:
    if any(len(group) > 1 for group in orphans.values()):
      raise WorkerError(
        "ambiguous_pdf_form", "orphan widgets share a name and cannot be safely reattached"
      )
    try:
      repair = PdfWriter()
      repair.clone_document_from_reader(reader)
      repair.reattach_fields()
      fields = repair.get_fields() or {}
      # Keep the repaired writer's reader-backed clone as the source for subsequent fill.
      reader = repair
      widgets = _widgets(repair)
    except Exception as exc:
      raise WorkerError(
        "invalid_pdf_template", "orphan form widgets could not be reattached"
      ) from exc
  return reader, fields, widgets


def _field_type(field: Any) -> str:
  field_type = field.get("/FT")
  flags = int(field.get("/Ff", 0))
  if field_type == "/Tx":
    return "text"
  if field_type == "/Ch":
    return "choice"
  if field_type == "/Btn":
    return "radio" if flags & (1 << 15) else "checkbox"
  if field_type == "/Sig":
    return "signature"
  return "unsupported"


def _value_is_populated(field: Any, value: Any) -> bool:
  if value is None:
    return False
  if _field_type(field) in {"checkbox", "radio"}:
    return str(value) not in {"Off", "/Off", "None"}
  return bool(str(value))


def _field_is_populated(field: Any) -> bool:
  return _value_is_populated(field, field.get("/V"))


def _button_states(field: Any, widgets: list[Any], name: str) -> set[str]:
  states: set[str] = set()
  for widget in widgets:
    if _qualified_name(widget) != name:
      continue
    appearance = widget.get("/AP")
    normal = appearance.get("/N") if appearance else None
    if normal:
      for state in normal.get_object():
        state = str(state)
        if state != "/Off":
          states.add(state)
  return states


def _choice_values(field: Any) -> set[str]:
  options = field.get("/Opt", [])
  values: set[str] = set()
  for option in options:
    if isinstance(option, (list, tuple)):
      option = option[0]
    values.add(str(option))
  return values


def _validate_values(fields: dict[str, Any], widgets: list[Any], requested: Any) -> dict[str, str]:
  if not isinstance(requested, dict):
    raise WorkerError("invalid_form_value", "field_values must be an object")
  unknown = set(requested) - set(fields)
  if unknown:
    raise WorkerError("unknown_form_field", f"unknown form fields: {sorted(unknown)}")
  for name, field in fields.items():
    if _field_type(field) == "signature" and field.get("/V"):
      raise WorkerError("signed_pdf_template", "signed PDF forms cannot be modified")
    if name in requested and _field_type(field) == "signature":
      raise WorkerError("unsupported_form_field", f"signature field {name!r} cannot be filled")
    if name in requested and _field_type(field) == "unsupported":
      raise WorkerError(
        "unsupported_form_field", f"field {name!r} uses an unsupported PDF field type"
      )
  values: dict[str, str] = {}
  for name, value in requested.items():
    field = fields[name]
    kind = _field_type(field)
    if kind == "text":
      if not isinstance(value, str) or "\x00" in value:
        raise WorkerError("invalid_form_value", f"field {name!r} requires text")
      maximum = field.get("/MaxLen")
      if maximum is not None and len(value) > int(maximum):
        raise WorkerError("invalid_form_value", f"field {name!r} exceeds its maximum length")
      values[name] = value
    elif kind == "choice":
      if not isinstance(value, str) or "\x00" in value or value not in _choice_values(field):
        raise WorkerError(
          "invalid_form_value", f"field {name!r} requires one of its declared choices"
        )
      values[name] = value
    elif kind == "checkbox":
      if not isinstance(value, bool):
        raise WorkerError("invalid_form_value", f"checkbox {name!r} requires a boolean")
      states = _button_states(field, widgets, name)
      if value and len(states) != 1:
        raise WorkerError("invalid_pdf_template", f"checkbox {name!r} has an ambiguous on-state")
      values[name] = next(iter(states)) if value else "/Off"
    elif kind == "radio":
      states = _button_states(field, widgets, name)
      if not isinstance(value, str) or "/" + value not in states:
        raise WorkerError(
          "invalid_form_value", f"radio field {name!r} requires one of its declared options"
        )
      values[name] = "/" + value
    else:
      raise WorkerError(
        "unsupported_form_field", f"field {name!r} uses an unsupported PDF field type"
      )
  return values


def _effective_value(widget: Any) -> Any:
  parent = widget.get("/Parent")
  if parent:
    return parent.get_object().get("/V")
  return widget.get("/V")


def _prepare_flatten_appearances(writer: PdfWriter) -> None:
  acro_form = writer.root_object.get("/AcroForm")
  if not acro_form:
    return
  defaults = acro_form.get_object().get("/DR")
  fonts = defaults.get_object().get("/Font") if defaults else None
  if fonts is None:
    return
  for widget in _widgets(writer):
    appearance = widget.get("/AP")
    normal = appearance.get("/N") if appearance else None
    normal_object = normal.get_object() if normal and hasattr(normal, "get_object") else normal
    if (
      normal_object and not hasattr(normal_object, "get_data") and hasattr(normal_object, "values")
    ):
      streams = normal_object.values()
    else:
      streams = [normal]
    for stream_ref in streams:
      stream = (
        stream_ref.get_object() if stream_ref and hasattr(stream_ref, "get_object") else stream_ref
      )
      if not stream or not hasattr(stream, "get_data"):
        continue
      resources = stream.get("/Resources")
      if resources is None:
        resources = DictionaryObject()
        stream[NameObject("/Resources")] = resources
      resources = resources.get_object() if hasattr(resources, "get_object") else resources
      if resources.get("/Font") is None:
        resources[NameObject("/Font")] = fonts


def _flatten_widgets(writer: PdfWriter) -> None:
  resource_number = 0
  for page in writer.pages:
    page_resources_ref = page.get("/Resources")
    page_resources = page_resources_ref.get_object() if page_resources_ref else DictionaryObject()
    if page.get("/Resources") is None:
      page[NameObject("/Resources")] = page_resources
    xobjects_ref = page_resources.get("/XObject")
    xobjects = xobjects_ref.get_object() if xobjects_ref else DictionaryObject()
    if xobjects_ref is None:
      page_resources[NameObject("/XObject")] = xobjects
    commands: list[str] = []
    for annotation_ref in page.get("/Annots", []):
      widget = annotation_ref.get_object()
      if widget.get("/Subtype") != "/Widget":
        continue
      appearance = widget.get("/AP")
      normal = appearance.get("/N") if appearance else None
      if normal is None:
        raise WorkerError("form_fill_failed", "cannot flatten a widget without a normal appearance")
      normal_object = normal.get_object() if hasattr(normal, "get_object") else normal
      if hasattr(normal_object, "get_data"):
        appearance_stream = normal_object
      else:
        state = widget.get("/AS", NameObject("/Off"))
        appearance_ref = normal_object.get(state)
        if appearance_ref is None:
          raise WorkerError(
            "form_fill_failed", "cannot find the active appearance for a button widget"
          )
        appearance_stream = (
          appearance_ref.get_object() if hasattr(appearance_ref, "get_object") else appearance_ref
        )
      if not hasattr(appearance_stream, "get_data"):
        raise WorkerError("form_fill_failed", "PDF widget appearance is not a stream")
      x0, y0 = widget["/Rect"][0], widget["/Rect"][1]
      resource_name = NameObject(f"/OrgDocWidget{resource_number}")
      resource_number += 1
      xobjects[resource_name] = writer._add_object(appearance_stream)
      commands.append(f"q\n1 0 0 1 {float(x0):g} {float(y0):g} cm\n{resource_name} Do\nQ\n")
    if not commands:
      continue
    flattened_content = DecodedStreamObject()
    flattened_content.set_data("".join(commands).encode("ascii"))
    content_ref = writer._add_object(flattened_content)
    existing_contents = page.get("/Contents")
    if existing_contents is None:
      page[NameObject("/Contents")] = content_ref
    elif isinstance(existing_contents, ArrayObject):
      existing_contents.append(content_ref)
    else:
      page[NameObject("/Contents")] = ArrayObject([existing_contents, content_ref])


def _verify_filled(path: Path, expected: dict[str, str], flattened: bool) -> dict[str, Any]:
  reader = PdfReader(path, strict=True)
  if flattened:
    if reader.get_fields() is not None or "/AcroForm" in reader.trailer["/Root"].get_object():
      raise WorkerError("form_fill_failed", "flattened PDF still contains an AcroForm field tree")
    if any(widget.get("/Subtype") == "/Widget" for widget in _widgets(reader)):
      raise WorkerError("form_fill_failed", "flattened PDF still contains widget annotations")
    return {}
  fields = reader.get_fields() or {}
  widgets = _widgets(reader)
  for name, value in expected.items():
    field = fields.get(name)
    if field is None or str(field.get("/V")) != value:
      raise WorkerError(
        "form_fill_failed", f"field {name!r} was not stored with its requested value"
      )
    matching = [widget for widget in widgets if _qualified_name(widget) == name]
    if not matching:
      raise WorkerError(
        "form_fill_failed", f"field {name!r} has no widget annotation after filling"
      )
    for widget in matching:
      if str(_effective_value(widget)) != value:
        raise WorkerError(
          "form_fill_failed", f"widget value for field {name!r} does not match the field tree"
        )
      appearance = widget.get("/AP")
      if not appearance or not appearance.get("/N"):
        raise WorkerError("form_fill_failed", f"widget for field {name!r} has no appearance stream")
  return fields


def fill(request: dict[str, Any]) -> dict[str, Any]:
  temporary_dir: Path | None = None
  try:
    payload = request.get("input")
    if not isinstance(payload, dict):
      raise WorkerError("invalid_request", "input must be an object")
    document, _ = _request_context(request, "pdf-form-fill")
    template = _resolve_template(
      {**document, "organization_root": payload.get("organization_root")}
    )
    source, fields, widgets = _load_form(template)
    requested = _validate_values(fields, widgets, document.get("field_values", {}))
    flatten = document.get("flatten", False)
    if not isinstance(flatten, bool):
      raise WorkerError("invalid_request", "flatten must be true or false")
    filename = "completed-form.pdf"
    output, staged = _artifact_path(request, filename)
    temporary_dir = staged.parent
    writer = source if isinstance(source, PdfWriter) else PdfWriter(clone_from=source)
    if flatten:
      existing_fields = writer.get_fields() or {}
      values_to_write = {
        name: field.get("/V", "/Off" if field.get("/FT") == "/Btn" else "")
        for name, field in existing_fields.items()
      }
      values_to_write.update(requested)
      _prepare_flatten_appearances(writer)
    else:
      values_to_write = requested
    writer.update_page_form_field_values(None, values_to_write, auto_regenerate=False)
    if flatten:
      _prepare_flatten_appearances(writer)
      _flatten_widgets(writer)
      writer.remove_annotations(subtypes="/Widget")
      writer.root_object.pop(NameObject("/AcroForm"), None)
    with staged.open("wb") as stream:
      writer.write(stream)
    verified_fields = _verify_filled(staged, requested, flatten)
    os.replace(staged, output)
    if flatten:
      summary = [
        {
          "name": name,
          "type": _field_type(fields[name]),
          "populated": _value_is_populated(
            fields[name], values_to_write.get(name, fields[name].get("/V"))
          ),
        }
        for name in fields
      ]
    else:
      summary = [
        {
          "name": name,
          "type": _field_type(verified_fields[name]),
          "populated": _field_is_populated(verified_fields[name]),
        }
        for name in fields
      ]
    return _success(request, filename, summary)
  except WorkerError as exc:
    return _failed(request, exc.code, str(exc))
  except Exception as exc:
    return _failed(request, "form_fill_failed", f"PDF form fill failed ({type(exc).__name__})")
  finally:
    if temporary_dir is not None:
      import shutil

      shutil.rmtree(temporary_dir, ignore_errors=True)


def _main(operation: str) -> int:
  try:
    request = json.load(sys.stdin)
    if not isinstance(request, dict):
      raise ValueError("worker request must be a JSON object")
    result = generate(request) if operation == "generate" else fill(request)
  except Exception as exc:
    result = _failed({}, "invalid_request", str(exc))
  json.dump(result, sys.stdout, ensure_ascii=False, separators=(",", ":"))
  sys.stdout.write("\n")
  return 0


def main_generate() -> int:
  return _main("generate")


def main_fill() -> int:
  return _main("fill")


if __name__ == "__main__":
  operation = sys.argv[1] if len(sys.argv) > 1 else ""
  raise SystemExit(_main(operation))
