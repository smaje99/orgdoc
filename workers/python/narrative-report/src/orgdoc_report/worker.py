from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

PROTOCOL_VERSION = 1


def _escape(value: Any) -> str:
    return str(value).replace("\\", "\\\\").replace("`", "\\`").replace("[", "\\[").replace("]", "\\]")


def _yaml_string(value: Any) -> str:
    return json.dumps(str(value), ensure_ascii=False)


def render(request: dict[str, Any]) -> dict[str, Any]:
    try:
        payload = request["input"]
        document = payload["document"]
        context = payload["context"]
        if not isinstance(document, dict) or not isinstance(context, dict):
            raise ValueError("input must contain document and context objects")
        if not document.get("title") or not document.get("executive_summary") or not document.get("findings"):
            raise ValueError("title, executive_summary, and findings are required")

        brand = context["brand"]
        operation = context["operation"]
        profile = context["document_type"]
        colors = brand.get("visual", {}).get("colors", {})
        voice = brand.get("voice", {})
        artifact = Path(request["artifact_dir"]) / "report.md"
        artifact.parent.mkdir(parents=True, exist_ok=True)

        lines = [
            "---",
            f"title: {_yaml_string(document['title'])}",
            f"organization: {_yaml_string(context['organization']['name'])}",
            f"brand: {_yaml_string(brand['name'])}",
            f"brand_id: {_yaml_string(brand['id'])}",
            f"brand_version: {_yaml_string(brand['version'])}",
            f"primary_color: {_yaml_string(colors.get('primary', ''))}",
            f"operation: {_yaml_string(operation['name'])}",
            f"document_type: {_yaml_string(profile['id'])}",
            f"language: {_yaml_string(voice.get('language', 'es'))}",
            f"voice_tone: {_yaml_string(voice.get('tone', ''))}",
            "---",
            "",
            f"# {document['title']}",
            "",
            "## Resumen ejecutivo",
            "",
            document["executive_summary"].strip(),
            "",
            "## Hallazgos",
            "",
        ]
        for finding in document["findings"]:
            lines.extend([f"### {finding['title']}", "", "**Evidencia**", "", finding["evidence"].strip(), "", "**Análisis**", "", finding["analysis"].strip(), ""])
        references = document.get("source_references", [])
        if references:
            lines.extend(["## Referencias de origen", ""])
            lines.extend(f"- `{_escape(reference)}`" for reference in references)
            lines.append("")
        quotations = document.get("quotations", [])
        if quotations:
            lines.extend(["## Citas", ""])
            for quotation in quotations:
                lines.extend([f"> {quotation.strip()}", ""])
        citations = document.get("citations", [])
        if citations:
            lines.extend(["## Fuentes", ""])
            lines.extend(f"- {citation.strip()}" for citation in citations)
            lines.append("")
        artifact.write_text("\n".join(lines).rstrip() + "\n", encoding="utf-8")
        return {"protocol_version": PROTOCOL_VERSION, "job_id": request.get("job_id", ""), "status": "succeeded", "outputs": [{"path": "report.md", "media_type": "text/markdown"}], "diagnostics": []}
    except Exception as exc:
        return {"protocol_version": PROTOCOL_VERSION, "job_id": request.get("job_id", ""), "status": "failed", "outputs": [], "diagnostics": [{"code": "report_render_error", "message": str(exc)}]}


def main() -> int:
    try:
        request = json.load(sys.stdin)
        result = render(request)
    except Exception as exc:
        result = {"protocol_version": PROTOCOL_VERSION, "job_id": "", "status": "failed", "outputs": [], "diagnostics": [{"code": "invalid_request", "message": str(exc)}]}
    json.dump(result, sys.stdout, ensure_ascii=False, separators=(",", ":"))
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
