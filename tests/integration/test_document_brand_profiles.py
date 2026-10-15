from __future__ import annotations

import json
import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
ORG = ROOT / "examples" / "organization-docs"
INVOICE_WORKER = ROOT / "workers" / "python" / "reportlab-invoice"
REPORT_WORKER = ROOT / "workers" / "python" / "narrative-report"
INVOICE_FIXTURE = ROOT / "tests" / "fixtures" / "documents" / "invoice.json"
REPORT_FIXTURE = ROOT / "tests" / "fixtures" / "documents" / "report.json"


class DocumentBrandProfilesTest(unittest.TestCase):
  @classmethod
  def setUpClass(cls) -> None:
    cls.binary_dir = Path(tempfile.mkdtemp(prefix="orgdoc-brand-test-bin-"))
    cls.binary = cls.binary_dir / "orgdoc"
    subprocess.run(["go", "build", "-o", str(cls.binary), "./cmd/orgdoc"], cwd=ROOT, check=True)
    cls.addClassCleanup(shutil.rmtree, cls.binary_dir, ignore_errors=True)

  def test_invoice_and_report_workers_use_each_operation_brand(self) -> None:
    with tempfile.TemporaryDirectory(prefix="orgdoc-brands-") as temp:
      root = Path(temp)
      env = os.environ.copy()
      env["PATH"] = os.pathsep.join(
        [
          str(INVOICE_WORKER / ".venv" / "bin"),
          str(REPORT_WORKER / ".venv" / "bin"),
          env.get("PATH", ""),
        ]
      )
      invoice = root / "invoice.json"
      invoice.write_text(INVOICE_FIXTURE.read_text(encoding="utf-8"), encoding="utf-8")
      report = root / "report.json"
      report.write_text(REPORT_FIXTURE.read_text(encoding="utf-8"), encoding="utf-8")

      cases = [
        ("consulting", "invoice", invoice, "invoice.pdf", "Henko Technologies", "#173F5F"),
        ("academy", "invoice", invoice, "invoice.pdf", "Henko Academy", "#6B3FA0"),
        ("consulting", "report", report, "report.md", "Henko Technologies", "#173F5F"),
        ("academy", "report", report, "report.md", "Henko Academy", "#6B3FA0"),
      ]
      for index, (
        operation,
        document_type,
        input_path,
        filename,
        brand,
        primary_color,
      ) in enumerate(cases):
        with self.subTest(operation=operation, document_type=document_type):
          job_id = f"brand-{index}"
          rendered = subprocess.run(
            [
              str(self.binary),
              "document",
              "render",
              "--organization",
              str(ORG),
              "--operation",
              operation,
              "--document-type",
              document_type,
              "--input",
              str(input_path),
              "--output-root",
              str(root / "runs"),
              "--job-id",
              job_id,
              "--format",
              "json",
            ],
            cwd=ROOT,
            env=env,
            text=True,
            capture_output=True,
            check=False,
          )
          self.assertEqual(rendered.returncode, 0, rendered.stderr)
          result = json.loads(rendered.stdout)
          self.assertEqual(result["context"]["brand"]["name"], brand)
          self.assertEqual(
            result["context"]["brand"]["visual"]["colors"]["primary"].lower(), primary_color.lower()
          )
          artifact = root / "runs" / job_id / filename
          self.assertTrue(artifact.is_file())
          if filename == "report.md":
            content = artifact.read_text(encoding="utf-8")
            self.assertIn(brand, content)
            self.assertIn("El equipo completó", content)
            self.assertIn(f'primary_color: "{primary_color}"', content)
          else:
            extracted = subprocess.run(
              ["pdftotext", str(artifact), "-"], check=True, capture_output=True, text=True
            ).stdout
            self.assertIn(brand, extracted)
            prefix = root / f"{job_id}-page"
            subprocess.run(
              [
                "pdftoppm",
                "-f",
                "1",
                "-singlefile",
                "-scale-to",
                "1000",
                "-png",
                str(artifact),
                str(prefix),
              ],
              check=True,
            )
            red, green, blue = tuple(
              int(primary_color[index : index + 2], 16) for index in (1, 3, 5)
            )
            pixel_count = subprocess.run(
              [
                str(INVOICE_WORKER / ".venv" / "bin" / "python"),
                "-c",
                "\n".join(
                  [
                    "from collections import Counter",
                    "from PIL import Image",
                    "import sys",
                    "image = Image.open(sys.argv[1]).convert('RGB')",
                    "counts = Counter(image.getdata())",
                    "print(counts[tuple(map(int, sys.argv[2:5]))])",
                  ]
                ),
                str(prefix.with_suffix(".png")),
                str(red),
                str(green),
                str(blue),
              ],
              check=True,
              capture_output=True,
              text=True,
            ).stdout.strip()
            self.assertGreater(
              int(pixel_count),
              100,
              f"{brand} primary color should be present in the rendered invoice",
            )


if __name__ == "__main__":
  unittest.main()
