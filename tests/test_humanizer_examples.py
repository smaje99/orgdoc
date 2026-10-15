import json
import unittest
from pathlib import Path

FIXTURES = Path(__file__).parent / "fixtures" / "humanizer"


class HumanizerExamplesTest(unittest.TestCase):
  def load_pair(self, name: str) -> tuple[dict[str, str], dict[str, str]]:
    before = json.loads((FIXTURES / f"{name}-before.json").read_text(encoding="utf-8"))
    after = json.loads((FIXTURES / f"{name}-after.json").read_text(encoding="utf-8"))
    return before, after

  def test_invoice_description_changes_without_changing_protected_bytes(self) -> None:
    before, after = self.load_pair("invoice")
    self.assertNotEqual(before["description"], after["description"])
    self.assertEqual(
      before["protected_fields_json"].encode("utf-8"),
      after["protected_fields_json"].encode("utf-8"),
    )

  def test_report_prose_changes_without_changing_sources_or_citation_bytes(self) -> None:
    before, after = self.load_pair("report")
    self.assertNotEqual(before["executive_summary"], after["executive_summary"])
    self.assertEqual(
      before["protected_fields_json"].encode("utf-8"),
      after["protected_fields_json"].encode("utf-8"),
    )


if __name__ == "__main__":
  unittest.main()
