from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
ORG = ROOT / "examples" / "organization-docs"
WORKER = ROOT / "workers" / "python" / "reportlab-invoice"
FIXTURE = ROOT / "tests" / "fixtures" / "documents" / "invoice.json"


class InvoiceWorkerContractTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.binary_dir = Path(tempfile.mkdtemp(prefix="orgdoc-test-bin-"))
        cls.binary = cls.binary_dir / "orgdoc"
        subprocess.run(["go", "build", "-o", str(cls.binary), "./cmd/orgdoc"], cwd=ROOT, check=True)
        cls.addClassCleanup(shutil.rmtree, cls.binary_dir, ignore_errors=True)

    def test_reportlab_invoice_worker_runs_through_orgdoc_contract(self) -> None:
        with tempfile.TemporaryDirectory(prefix="orgdoc-invoice-") as temp:
            root = Path(temp)
            request = root / "invoice.json"
            request.write_text(FIXTURE.read_text(encoding="utf-8"), encoding="utf-8")
            env = os.environ.copy()
            env["PATH"] = os.pathsep.join([str(WORKER / ".venv" / "bin"), env.get("PATH", "")])
            rendered = subprocess.run(
                [
                    str(self.binary), "document", "render", "--organization", str(ORG),
                    "--operation", "consulting", "--document-type", "invoice", "--input", str(request),
                    "--output-root", str(root / "runs"), "--job-id", "henko-invoice", "--format", "json",
                ],
                cwd=ROOT,
                env=env,
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertEqual(rendered.returncode, 0, rendered.stderr)
            result = json.loads(rendered.stdout)
            self.assertEqual(result["status"], "succeeded")
            artifact = root / "runs" / "henko-invoice" / "invoice.pdf"
            self.assertTrue(artifact.is_file())

            text = subprocess.run(["pdftotext", str(artifact), "-"], check=True, capture_output=True, text=True).stdout
            self.assertIn("HT-2026-051", text)
            self.assertIn("Henko Technologies", text)
            self.assertIn("416.500", text)

            info = subprocess.run(["pdfinfo", str(artifact)], check=True, capture_output=True, text=True).stdout
            self.assertRegex(info, r"Page size:\s+595\.2[0-9]* x 841\.8[0-9]* pts \(A4\)")
            self.assertRegex(info, r"Pages:\s+1\b")
            fonts = subprocess.run(["pdffonts", str(artifact)], check=True, capture_output=True, text=True).stdout
            self.assertIn("Helvetica", fonts, "built-in Helvetica is the fallback for profile fonts that are not packaged")
            image_prefix = root / "invoice-page"
            subprocess.run(["pdftoppm", "-f", "1", "-singlefile", "-scale-to", "1000", "-png", str(artifact), str(image_prefix)], check=True)
            self.assertTrue(image_prefix.with_suffix(".png").is_file())
            pixels = subprocess.run(
                [str(WORKER / ".venv" / "bin" / "python"), "-c", "from PIL import Image; from collections import Counter; import sys; im=Image.open(sys.argv[1]).convert('RGB'); c=Counter(im.getdata()); print(c[(23,63,95)])", str(image_prefix.with_suffix(".png"))],
                check=True,
                capture_output=True,
                text=True,
            ).stdout.strip()
            self.assertGreater(int(pixels), 300, "Henko primary color should be visible in the page and logo")

    def test_long_invoice_lines_flow_to_a_second_page_without_losing_the_last_line(self) -> None:
        with tempfile.TemporaryDirectory(prefix="orgdoc-invoice-overflow-") as temp:
            root = Path(temp)
            invoice = {
                "invoice_number": "HT-LONG-1",
                "issue_date": "2026-10-14",
                "customer": {"name": "Cliente de prueba"},
                "lines": [
                    {"description": f"Servicio de consultoría con alcance detallado {index:02d}: " + "revisión de procesos y documentación. " * 4, "quantity": 1, "unit_price": 10000, "tax_rate": 0}
                    for index in range(1, 26)
                ],
                "subtotal": 250000,
                "tax_total": 0,
                "grand_total": 250000,
                "currency": "COP",
            }
            request = root / "long-invoice.json"
            request.write_text(json.dumps(invoice), encoding="utf-8")
            env = os.environ.copy()
            env["PATH"] = os.pathsep.join([str(WORKER / ".venv" / "bin"), env.get("PATH", "")])
            rendered = subprocess.run(
                [str(self.binary), "document", "render", "--organization", str(ORG), "--operation", "consulting", "--document-type", "invoice", "--input", str(request), "--output-root", str(root / "runs"), "--job-id", "long-invoice", "--format", "json"],
                cwd=ROOT, env=env, text=True, capture_output=True, check=False,
            )
            self.assertEqual(rendered.returncode, 0, rendered.stderr)
            artifact = root / "runs" / "long-invoice" / "invoice.pdf"
            info = subprocess.run(["pdfinfo", str(artifact)], check=True, capture_output=True, text=True).stdout
            pages = re.search(r"Pages:\s+(\d+)", info)
            self.assertIsNotNone(pages, info)
            self.assertGreaterEqual(int(pages.group(1)), 2, "long invoice should flow across pages")
            extracted = subprocess.run(["pdftotext", str(artifact), "-"], check=True, capture_output=True, text=True).stdout
            self.assertIn("Servicio de consultoría con alcance detallado 25", extracted)
            self.assertIn("COP 250.000", extracted)


if __name__ == "__main__":
    unittest.main()
