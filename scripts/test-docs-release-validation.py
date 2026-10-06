#!/usr/bin/env python3
"""Keep historical release evidence distinct from current design."""
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("facts", Path(__file__).with_name("check-docs-facts.py"))
facts = importlib.util.module_from_spec(spec)
spec.loader.exec_module(facts)


class DocumentationStatusTest(unittest.TestCase):
    def test_current_design_cannot_be_reclassified_as_history(self):
        path = facts.ROOT / "docs/current-design.md"
        with self.assertRaisesRegex(AssertionError, "not registered"):
            facts.validate_documentation_status(path, "> 状态：历史记录 · 2026-10-06\n")
        self.assertEqual(facts.validate_documentation_status(path, "> 状态：已实现\n"), "已实现")

    def test_registered_history_requires_date_and_classification(self):
        path = facts.ROOT / facts.HISTORICAL_DOCS[0]
        with self.assertRaisesRegex(AssertionError, "classified as historical"):
            facts.validate_documentation_status(path, "> 状态：已实现 · 2026-10-06\n")
        with self.assertRaisesRegex(AssertionError, "no date"):
            facts.validate_documentation_status(path, "> 状态：历史记录\n")
        self.assertEqual(facts.validate_documentation_status(path, "> 状态：历史记录 · 2026-10-06\n"), "历史记录")

    def test_missing_or_duplicate_status_is_rejected(self):
        path = facts.ROOT / "docs/current-design.md"
        for text in ("# No status\n", "> 状态：已实现\n> 状态：规划改造\n"):
            with self.assertRaisesRegex(AssertionError, "exactly one"):
                facts.validate_documentation_status(path, text)


if __name__ == "__main__":
    unittest.main()
