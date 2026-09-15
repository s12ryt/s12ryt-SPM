import unittest
from verify_go_tests import REQUIRED, verify


class VerifyGoTests(unittest.TestCase):
    def setUp(self):
        self.passed = [dict(Action='pass', Package=p, Test=t) for p, t in REQUIRED]

    def test_required_tests_pass(self):
        verify(self.passed)

    def test_package_without_unit_tests_is_not_a_skipped_test(self):
        verify(self.passed + [dict(Action='skip', Package='spm/cmd/agent', Elapsed=0)])

    def test_skipped_test_case_is_rejected(self):
        with self.assertRaises(ValueError):
            verify(self.passed + [dict(Action='skip', Package='spm/internal/model', Test='TestUnexpected')])

    def test_missing_postgres_pass_is_rejected(self):
        with self.assertRaises(ValueError):
            verify([e for e in self.passed if e['Test'] != 'TestPostgresIntegration'])

    def test_failed_package_is_rejected_even_with_required_passes(self):
        with self.assertRaises(ValueError):
            verify(self.passed + [dict(Action='fail', Package='spm/internal/model')])
