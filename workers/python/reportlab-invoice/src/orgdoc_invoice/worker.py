from __future__ import annotations

import json
import sys
from decimal import Decimal, InvalidOperation, ROUND_HALF_UP
from pathlib import Path
from typing import Any
from xml.sax.saxutils import escape

PROTOCOL_VERSION = 1


def _money(value: Decimal, currency: str) -> str:
    value = value.quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)
    if value == value.to_integral_value():
        formatted = f"{int(value):,}".replace(",", ".")
    else:
        formatted = f"{value:,.2f}".replace(",", "_").replace(".", ",").replace("_", ".")
    return f"{currency} {formatted}"


def _decimal(value: Any, field: str) -> Decimal:
    try:
        result = Decimal(str(value))
    except (InvalidOperation, TypeError, ValueError) as exc:
        raise ValueError(f"{field} must be a decimal number") from exc
    if not result.is_finite():
        raise ValueError(f"{field} must be finite")
    return result


def _color(value: Any, fallback: str):
    from reportlab.lib import colors

    try:
        return colors.HexColor(str(value)) if value else colors.HexColor(fallback)
    except (ValueError, TypeError):
        return colors.HexColor(fallback)


def _within(root: Path, relative: str) -> Path:
    if Path(relative).is_absolute():
        raise ValueError("brand asset paths must be relative to the organization root")
    base = root.resolve()
    target = (base / relative).resolve(strict=True)
    if not target.is_file() or not target.is_relative_to(base):
        raise ValueError("brand asset must be a regular file inside the organization root")
    return target


def _validate_totals(document: dict[str, Any]) -> tuple[Decimal, Decimal, Decimal]:
    currency = document.get("currency")
    if not isinstance(currency, str) or len(currency) != 3 or not currency.isalpha() or not currency.isupper():
        raise ValueError("currency must be a three-letter uppercase code")
    lines = document.get("lines")
    if not isinstance(lines, list) or not lines:
        raise ValueError("lines must contain at least one invoice line")
    subtotal = Decimal("0")
    tax_total = Decimal("0")
    for index, line in enumerate(lines):
        quantity = _decimal(line.get("quantity"), f"lines[{index}].quantity")
        unit_price = _decimal(line.get("unit_price"), f"lines[{index}].unit_price")
        tax_rate = _decimal(line.get("tax_rate"), f"lines[{index}].tax_rate")
        if quantity <= 0 or unit_price < 0 or tax_rate < 0:
            raise ValueError(f"lines[{index}] has a negative amount or non-positive quantity")
        line_total = (quantity * unit_price).quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)
        if "total" in line and _decimal(line["total"], f"lines[{index}].total") != line_total:
            raise ValueError(f"lines[{index}].total does not match quantity times unit_price")
        subtotal += line_total
        tax_total += (line_total * tax_rate).quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)
    tax_total = tax_total.quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)
    requested_subtotal = _decimal(document.get("subtotal"), "subtotal")
    requested_tax = _decimal(document.get("tax_total"), "tax_total")
    requested_grand = _decimal(document.get("grand_total"), "grand_total")
    if subtotal != requested_subtotal or tax_total != requested_tax or subtotal + tax_total != requested_grand:
        raise ValueError("subtotal, tax_total, and grand_total do not match the invoice lines")
    return subtotal, tax_total, requested_grand


def render(request: dict[str, Any]) -> dict[str, Any]:
    try:
        from reportlab.lib import colors
        from reportlab.lib.enums import TA_RIGHT
        from reportlab.lib.pagesizes import A4
        from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
        from reportlab.lib.units import mm
        from reportlab.platypus import Image, KeepTogether, Paragraph, SimpleDocTemplate, Spacer, Table, TableStyle
        from svglib.svglib import svg2rlg
        from reportlab.graphics import renderPDF
    except ImportError as exc:
        return _failed(request, "worker_environment_missing", f"ReportLab invoice dependencies are missing; sync workers/python/reportlab-invoice with uv. ({exc.name})")

    try:
        payload = request["input"]
        document = payload["document"]
        context = payload["context"]
        if not isinstance(document, dict) or not isinstance(context, dict):
            raise ValueError("input must contain document and context objects")
        customer = document["customer"]
        lines = document["lines"]
        subtotal, tax_total, grand_total = _validate_totals(document)
        colors_profile = context["brand"].get("visual", {}).get("colors", {})
        primary = _color(colors_profile.get("primary"), "#173F5F")
        secondary = _color(colors_profile.get("secondary"), "#20639B")
        accent = _color(colors_profile.get("accent"), "#F6D55C")
        text_color = _color(colors_profile.get("text"), "#17212B")
        muted = colors.HexColor("#586B7B")
        styles = getSampleStyleSheet()
        title = ParagraphStyle("OrgDocTitle", parent=styles["Title"], fontName="Helvetica-Bold", fontSize=19, leading=23, textColor=primary, alignment=0, spaceAfter=3)
        section = ParagraphStyle("OrgDocSection", parent=styles["Heading2"], fontName="Helvetica-Bold", fontSize=10, leading=13, textColor=primary, spaceBefore=9, spaceAfter=5)
        body = ParagraphStyle("OrgDocBody", parent=styles["BodyText"], fontName="Helvetica", fontSize=8.5, leading=12, textColor=text_color)
        small = ParagraphStyle("OrgDocSmall", parent=body, fontSize=7.5, leading=10, textColor=muted)
        right = ParagraphStyle("OrgDocRight", parent=body, alignment=TA_RIGHT)
        header = ParagraphStyle("OrgDocHeader", parent=body, fontName="Helvetica-Bold", textColor=colors.white, fontSize=8)
        total_style = ParagraphStyle("OrgDocTotal", parent=body, fontName="Helvetica-Bold", fontSize=10, textColor=primary)

        artifact_dir = Path(request["artifact_dir"])
        output = artifact_dir / "invoice.pdf"
        output.parent.mkdir(parents=True, exist_ok=True)

        def footer(canvas, doc):
            canvas.saveState()
            canvas.setStrokeColor(accent)
            canvas.setLineWidth(1.5)
            canvas.line(doc.leftMargin, 12 * mm, A4[0] - doc.rightMargin, 12 * mm)
            canvas.setFillColor(muted)
            canvas.setFont("Helvetica", 7)
            canvas.drawString(doc.leftMargin, 8 * mm, str(context["operation"].get("legal_name") or context["brand"]["name"]))
            canvas.drawRightString(A4[0] - doc.rightMargin, 8 * mm, f"{document['invoice_number']} · {doc.page}")
            canvas.restoreState()

        doc = SimpleDocTemplate(str(output), pagesize=A4, rightMargin=18 * mm, leftMargin=18 * mm, topMargin=15 * mm, bottomMargin=19 * mm, title=f"Factura {document['invoice_number']}", author=str(context["brand"]["name"]))
        story = []
        logo = None
        for asset in context["brand"].get("assets", []):
            if str(asset).lower().endswith(".svg"):
                try:
                    drawing = svg2rlg(str(_within(Path(payload["organization_root"]), asset)))
                    if drawing:
                        factor = min(150 / drawing.width, 42 / drawing.height)
                        drawing.width *= factor
                        drawing.height *= factor
                        drawing.scale(factor, factor)
                        logo = drawing
                        break
                except (OSError, ValueError, TypeError):
                    continue
            elif str(asset).lower().endswith((".png", ".jpg", ".jpeg")):
                try:
                    logo = Image(str(_within(Path(payload["organization_root"]), asset)), width=145, height=42, kind="proportional")
                    break
                except (OSError, ValueError, TypeError):
                    continue
        if logo:
            story.extend([logo, Spacer(1, 7)])
        else:
            story.extend([Paragraph(escape(str(context["brand"]["name"])), title), Spacer(1, 5)])

        stripe = Table([[""]], colWidths=[doc.width], rowHeights=[2.2])
        stripe.setStyle(TableStyle([("BACKGROUND", (0, 0), (-1, -1), accent), ("LEFTPADDING", (0, 0), (-1, -1), 0), ("RIGHTPADDING", (0, 0), (-1, -1), 0), ("TOPPADDING", (0, 0), (-1, -1), 0), ("BOTTOMPADDING", (0, 0), (-1, -1), 0)]))
        story.extend([stripe, Spacer(1, 10), Paragraph("FACTURA", title)])
        issuer = context["operation"]
        issuer_name = issuer.get("legal_name") or context["brand"]["name"]
        issuer_lines = [Paragraph(escape(str(issuer_name)), ParagraphStyle("IssuerName", parent=body, fontName="Helvetica-Bold"))]
        if issuer.get("tax_id"):
            issuer_lines.append(Paragraph(f"NIT: {escape(str(issuer['tax_id']))}", body))
        if issuer.get("address"):
            issuer_lines.append(Paragraph(escape(str(issuer["address"])), body))
        story.extend([Paragraph("EMITIDO POR", section), Table([[issuer_lines]], colWidths=[doc.width], style=TableStyle([("BACKGROUND", (0, 0), (-1, -1), colors.HexColor("#F3F6F8")), ("LEFTPADDING", (0, 0), (-1, -1), 9), ("RIGHTPADDING", (0, 0), (-1, -1), 9), ("TOPPADDING", (0, 0), (-1, -1), 7), ("BOTTOMPADDING", (0, 0), (-1, -1), 7)]))])
        invoice_meta = [
            [Paragraph(f"<b>Número</b><br/>{escape(str(document['invoice_number']))}", body), Paragraph(f"<b>Emisión</b><br/>{escape(str(document['issue_date']))}", body)],
            [Paragraph(f"<b>Vencimiento</b><br/>{escape(str(document.get('due_date', '—')))}", body), Paragraph(f"<b>Moneda</b><br/>{escape(document['currency'])}", body)],
        ]
        meta = Table(invoice_meta, colWidths=[doc.width / 2, doc.width / 2])
        meta.setStyle(TableStyle([("BACKGROUND", (0, 0), (-1, -1), colors.HexColor("#F3F6F8")), ("BOX", (0, 0), (-1, -1), 0.4, colors.HexColor("#D8E0E6")), ("INNERGRID", (0, 0), (-1, -1), 0.4, colors.HexColor("#D8E0E6")), ("VALIGN", (0, 0), (-1, -1), "TOP"), ("LEFTPADDING", (0, 0), (-1, -1), 9), ("RIGHTPADDING", (0, 0), (-1, -1), 9), ("TOPPADDING", (0, 0), (-1, -1), 7), ("BOTTOMPADDING", (0, 0), (-1, -1), 7)]))
        story.extend([Spacer(1, 6), meta, Paragraph("FACTURAR A", section)])
        customer_rows = [[Paragraph(escape(str(customer.get("name", ""))), ParagraphStyle("CustomerName", parent=body, fontName="Helvetica-Bold"))]]
        for key in ("tax_id", "address"):
            if customer.get(key):
                customer_rows.append([Paragraph(escape(str(customer[key])), body)])
        customer_table = Table(customer_rows, colWidths=[doc.width])
        customer_table.setStyle(TableStyle([("BACKGROUND", (0, 0), (-1, -1), colors.HexColor("#F3F6F8")), ("LEFTPADDING", (0, 0), (-1, -1), 9), ("RIGHTPADDING", (0, 0), (-1, -1), 9), ("TOPPADDING", (0, 0), (-1, -1), 4), ("BOTTOMPADDING", (0, 0), (-1, -1), 4)]))
        story.extend([customer_table, Paragraph("DETALLE", section)])
        rows = [[Paragraph("Descripción", header), Paragraph("Cantidad", header), Paragraph("Valor unitario", header), Paragraph("Total", header)]]
        for line in lines:
            quantity = _decimal(line["quantity"], "quantity")
            rate = _decimal(line["unit_price"], "unit_price")
            amount = (quantity * rate).quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)
            rows.append([Paragraph(escape(str(line["description"])), body), Paragraph(escape(str(line["quantity"])), right), Paragraph(_money(rate, document["currency"]), right), Paragraph(_money(amount, document["currency"]), right)])
        table = Table(rows, colWidths=[doc.width * 0.48, doc.width * 0.13, doc.width * 0.2, doc.width * 0.19], repeatRows=1, splitByRow=1, hAlign="LEFT")
        table.setStyle(TableStyle([("BACKGROUND", (0, 0), (-1, 0), primary), ("ROWBACKGROUNDS", (0, 1), (-1, -1), [colors.white, colors.HexColor("#F7F9FA")]), ("TEXTCOLOR", (0, 0), (-1, 0), colors.white), ("VALIGN", (0, 0), (-1, -1), "TOP"), ("GRID", (0, 0), (-1, -1), 0.35, colors.HexColor("#D8E0E6")), ("LEFTPADDING", (0, 0), (-1, -1), 7), ("RIGHTPADDING", (0, 0), (-1, -1), 7), ("TOPPADDING", (0, 0), (-1, -1), 7), ("BOTTOMPADDING", (0, 0), (-1, -1), 7)]))
        story.extend([table, Spacer(1, 8)])
        totals = [
            [Paragraph("Subtotal", body), Paragraph(_money(subtotal, document["currency"]), right)],
            [Paragraph("Impuestos", body), Paragraph(_money(tax_total, document["currency"]), right)],
            [Paragraph("Total", total_style), Paragraph(_money(grand_total, document["currency"]), ParagraphStyle("TotalRight", parent=total_style, alignment=TA_RIGHT))],
        ]
        totals_table = Table(totals, colWidths=[doc.width * 0.65, doc.width * 0.35], hAlign="RIGHT")
        totals_table.setStyle(TableStyle([("LINEABOVE", (0, 2), (-1, 2), 1.1, secondary), ("BACKGROUND", (0, 2), (-1, 2), colors.HexColor("#F3F6F8")), ("LEFTPADDING", (0, 0), (-1, -1), 8), ("RIGHTPADDING", (0, 0), (-1, -1), 8), ("TOPPADDING", (0, 0), (-1, -1), 6), ("BOTTOMPADDING", (0, 0), (-1, -1), 6)]))
        story.append(KeepTogether([totals_table]))
        doc.build(story, onFirstPage=footer, onLaterPages=footer)
        return _succeeded(request, "invoice.pdf", "application/pdf")
    except Exception as exc:
        return _failed(request, "invoice_render_error", str(exc))


def _succeeded(request: dict[str, Any], path: str, media_type: str) -> dict[str, Any]:
    return {"protocol_version": PROTOCOL_VERSION, "job_id": request.get("job_id", ""), "status": "succeeded", "outputs": [{"path": path, "media_type": media_type}], "diagnostics": []}


def _failed(request: dict[str, Any], code: str, message: str) -> dict[str, Any]:
    return {"protocol_version": PROTOCOL_VERSION, "job_id": request.get("job_id", ""), "status": "failed", "outputs": [], "diagnostics": [{"code": code, "message": message}]}


def main() -> int:
    try:
        request = json.load(sys.stdin)
        result = render(request)
    except Exception as exc:
        result = _failed({}, "invalid_request", str(exc))
    json.dump(result, sys.stdout, ensure_ascii=False, separators=(",", ":"))
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
