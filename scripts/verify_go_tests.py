"""Check that Linux CI actually ran its required integration tests."""
import json
import sys
from pathlib import Path

REQUIRED = {
    ('spm/internal/store', 'TestPostgresIntegration'),
    ('spm/internal/collector', 'TestUpload'),
    ('spm/internal/collector', 'TestRunUploadsAndStops'),
    ('spm/internal/server', 'TestWebhookPayloadAndFailure'),
}


def verify(events):
    passed = {(e.get('Package'), e.get('Test')) for e in events if e.get('Action') == 'pass'}
    if not REQUIRED <= passed:
        raise ValueError(f'Required tests did not PASS: {REQUIRED - passed}')
    if any(e.get('Action') == 'skip' and e.get('Test') for e in events):
        raise ValueError('Unexpected skipped test')
    if any(e.get('Action') == 'fail' for e in events):
        raise ValueError('Failed test or package')


if __name__ == '__main__':
    verify([json.loads(line) for line in Path(sys.argv[1]).read_text().splitlines()])
    print('Verified: PostgreSQL and previously blocked HTTP tests PASS; zero skipped test cases')
