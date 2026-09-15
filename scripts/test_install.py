"""Exercise the real Bash installer, isolating network and host service commands."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


@unittest.skipUnless(os.name == 'posix' and shutil.which('bash'), 'Linux Bash required')
class InstallerTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name)
        self.root = self.base / 'root'
        self.root.mkdir()
        self.commands = self.base / 'commands'
        self.commands.mkdir()
        self.log = self.base / 'calls'
        self.payload = b'prebuilt release binary\n'
        (self.base / 'payload').write_bytes(self.payload)
        self.script = Path(__file__).resolve().parents[1] / 'install.sh'
        self.env = dict(os.environ, PATH=f'{self.commands}:{os.environ["PATH"]}',
                        FIXTURE=str(self.base), MOCK_ARCH='x86_64', MOCK_OS='Linux',
                        SPM_ADMIN_USER='admin', SPM_ADMIN_PASSWORD='test-password-12345',
                        SPM_SERVER='https://monitor.example.com', SPM_NODE_ID='node-123',
                        SPM_TOKEN='fixture-token-secret', SPM_LISTEN='127.0.0.1:18080')
        self.command('uname', '[[ "$1" == -s ]] && echo "$MOCK_OS" || echo "$MOCK_ARCH"')
        self.command('id', '[[ "$1" == -u ]] && echo 0 || exit 0')
        self.command('getent', 'exit 0')
        self.command('useradd', 'echo "useradd $*" >> "$FIXTURE/calls"')
        self.command('chown', 'exit 0')
        self.command('systemctl', '''echo "systemctl $*" >> "$FIXTURE/calls"
if [[ "$*" == *is-active* && "${FAIL_SERVICE:-}" == 1 ]]; then exit 1; fi
exit 0''')
        self.command('curl', '''printf 'curl' >> "$FIXTURE/calls"
printf ' %s' "$@" >> "$FIXTURE/calls"
printf '\\n' >> "$FIXTURE/calls"
output=''; url=''
while (($#)); do
    case "$1" in
        -o|--output) output=$2; shift 2 ;;
        https://*) url=$1; shift ;;
        *) shift ;;
    esac
done
[[ "${FAIL_DOWNLOAD:-}" == 1 ]] && exit 22
case "$url" in
    https://github.com/s12ryt/s12ryt-SPM/releases/latest) printf 'https://github.com/s12ryt/s12ryt-SPM/releases/tag/v0.1.0' ;;
    https://github.com/s12ryt/s12ryt-SPM/releases/download/v0.1.0/SHA256SUMS)
        hash=$(sha256sum "$FIXTURE/payload"); hash=${hash%% *}
        [[ "${BAD_HASH:-}" == 1 ]] && hash=$(printf '%064d' 0)
        for role in server agent; do
            for arch in amd64 arm64; do
                printf '%s  spm-%s-linux-%s\\n' "$hash" "$role" "$arch"
            done
        done > "$output" ;;
    https://github.com/s12ryt/s12ryt-SPM/releases/download/v0.1.0/spm-*-linux-*) cp "$FIXTURE/payload" "$output" ;;
    *) echo "unexpected URL: $url" >&2; exit 23 ;;
esac''')

    def command(self, name, body):
        target = self.commands / name
        target.write_text('#!/usr/bin/env bash\nset -eu\n' + body + '\n')
        target.chmod(0o755)

    def run_installer(self, *args, **env):
        result = subprocess.run(['bash', '-c', 'set -euo pipefail; source "$1"; install_root="$2"; shift 2; main "$@"',
                                 'installer-test', str(self.script), str(self.root), *args],
                                env=dict(self.env, **env), stdin=subprocess.DEVNULL,
                                capture_output=True, text=True, timeout=15)
        self.output = result.stdout + result.stderr
        return result.returncode

    def installed(self, role='server'):
        return self.root / f'opt/spm/spm-{role}'

    def test_default_installs_only_server_from_pinned_release(self):
        self.assertEqual(self.run_installer(), 0, self.output)
        self.assertEqual(self.installed().read_bytes(), self.payload)
        self.assertFalse(self.installed('agent').exists())
        calls = self.log.read_text()
        self.assertIn('/releases/download/v0.1.0/spm-server-linux-amd64', calls)
        self.assertNotIn('spm-agent', calls)
        self.assertIn('systemctl restart spm-server', calls)
        config = self.root / 'etc/spm/server.env'
        self.assertEqual(config.stat().st_mode & 0o777, 0o600)
        self.assertIn('SPM_ADMIN_USER="admin"', config.read_text())
        self.assertNotIn(self.env['SPM_ADMIN_PASSWORD'], self.output)

    def test_agent_argument_installs_only_arm64_agent(self):
        self.assertEqual(self.run_installer('agent', '--version', 'v0.1.0', MOCK_ARCH='aarch64'), 0, self.output)
        self.assertFalse(self.installed().exists())
        self.assertEqual(self.installed('agent').read_bytes(), self.payload)
        calls = self.log.read_text()
        self.assertIn('/releases/download/v0.1.0/spm-agent-linux-arm64', calls)
        self.assertNotIn('spm-server', calls)
        self.assertNotIn('/releases/latest', calls)
        self.assertNotIn(self.env['SPM_TOKEN'], self.output)
        self.assertFalse((self.root / 'etc/spm/server.env').exists())

    def test_reinstall_preserves_configuration_and_data(self):
        config = self.root / 'etc/spm/server.env'
        config.parent.mkdir(parents=True)
        config.write_text('EXISTING="do not evaluate $(touch hacked)"\n')
        data = self.root / 'var/lib/spm/spm.db'
        data.parent.mkdir(parents=True)
        data.write_bytes(b'existing database')
        self.assertEqual(self.run_installer(SPM_ADMIN_PASSWORD=''), 0, self.output)
        self.assertEqual(config.read_text(), 'EXISTING="do not evaluate $(touch hacked)"\n')
        self.assertEqual(data.read_bytes(), b'existing database')

    def test_download_or_checksum_failure_keeps_old_binary(self):
        self.installed().parent.mkdir(parents=True)
        self.installed().write_bytes(b'old binary')
        for failure in ['FAIL_DOWNLOAD', 'BAD_HASH']:
            with self.subTest(failure=failure):
                self.assertNotEqual(self.run_installer(**{failure: '1'}), 0)
                self.assertEqual(self.installed().read_bytes(), b'old binary')
                calls = self.log.read_text() if self.log.exists() else ''
                self.assertNotIn('systemctl restart', calls)

    def test_invalid_input_and_missing_config_do_not_install(self):
        cases = [(('both',), {}), (('--version', '../main'), {}), ((), {'MOCK_ARCH': 'mips'}),
                 ((), {'MOCK_OS': 'Darwin'}), ((), {'SPM_ADMIN_PASSWORD': 'short'}),
                 (('agent',), {'SPM_TOKEN': ''}), (('agent',), {'SPM_SERVER': 'file:///etc/passwd'})]
        for args, env in cases:
            with self.subTest(args=args, env=env):
                self.assertNotEqual(self.run_installer(*args, **env), 0)
                self.assertFalse(self.installed().exists())
                self.assertFalse(self.installed('agent').exists())

    def test_service_failure_is_reported_and_restores_previous_binary(self):
        self.installed().parent.mkdir(parents=True)
        self.installed().write_bytes(b'old binary')
        unit = self.root / 'etc/systemd/system/spm-server.service'
        unit.parent.mkdir(parents=True)
        unit.write_text('[Service]\nEnvironment=EXISTING=1\n')
        self.assertNotEqual(self.run_installer(FAIL_SERVICE='1'), 0)
        self.assertIn('服務', self.output)
        self.assertEqual(self.installed().read_bytes(), b'old binary')
        self.assertEqual(unit.read_text(), '[Service]\nEnvironment=EXISTING=1\n')

    def test_file_install_failure_restores_previous_release(self):
        self.assertEqual(self.run_installer(), 0, self.output)
        self.installed().write_bytes(b'previous binary')
        unit = self.root / 'etc/systemd/system/spm-server.service'
        unit.write_text('[Service]\nEnvironment=ORIGINAL=1\n')
        real_install = shutil.which('install')
        self.command('install', f'''target="${{@: -1}}"
if [[ "$target" == *.service && ! -f "$FIXTURE/write-failed" ]]; then
    touch "$FIXTURE/write-failed"
    printf 'partial unit' > "$target"
    exit 73
fi
exec "{real_install}" "$@"''')
        self.assertEqual(self.run_installer(), 73, self.output)
        self.assertTrue((self.base / 'write-failed').exists(), self.output)
        self.assertEqual(self.installed().read_bytes(), b'previous binary')
        self.assertEqual(unit.read_text(), '[Service]\nEnvironment=ORIGINAL=1\n')

    def test_failed_first_install_can_retry_with_preserved_config(self):
        self.assertNotEqual(self.run_installer(FAIL_SERVICE='1'), 0)
        self.assertFalse(self.installed().exists())
        self.assertFalse((self.root / 'etc/systemd/system/spm-server.service').exists())
        config = self.root / 'etc/spm/server.env'
        original = config.read_bytes()
        self.assertEqual(self.run_installer(SPM_ADMIN_PASSWORD=''), 0, self.output)
        self.assertEqual(config.read_bytes(), original)
        self.assertEqual(self.installed().read_bytes(), self.payload)

    def test_environment_values_are_quoted_without_shell_execution(self):
        password = 'safe password " $HOME \\ still-secret'
        self.assertEqual(self.run_installer(SPM_ADMIN_PASSWORD=password), 0, self.output)
        text = (self.root / 'etc/spm/server.env').read_text()
        self.assertIn('\\"', text)
        self.assertIn('\\\\', text)
        self.assertIn('$HOME', text)
        self.assertNotIn(password, self.output)


if __name__ == '__main__':
    unittest.main()
