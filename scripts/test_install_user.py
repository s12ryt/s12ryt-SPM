"""User installer behavior with isolated HOME and mock OS/service boundaries."""
import os
from pathlib import Path
import shutil
import subprocess
import unittest
import test_install


@unittest.skipUnless(os.name == 'posix' and shutil.which('bash'), 'Linux Bash required')
class UserInstallerTests(unittest.TestCase):
    command = test_install.InstallerTests.command

    def setUp(self):
        test_install.InstallerTests.setUp(self)
        self.script = self.script.with_name('install-user.sh')
        self.home = self.base / 'home'
        self.home.mkdir()
        self.env.update(HOME=str(self.home), USER='fixture-user')
        self.log.touch()
        self.command('id', '[[ "$1" == -u ]] && echo "${MOCK_UID:-1000}" || echo fixture-user')
        self.command('systemctl', '''echo "systemctl $*" >> "$FIXTURE/calls"
[[ "$1" == --user ]] || exit 90
[[ "$*" == *show-environment* && "${NO_USER_SYSTEMD:-}" == 1 ]] && exit 1
[[ "$*" == *is-active* && "${FAIL_SERVICE:-}" == 1 ]] && exit 1
exit 0''')
        self.command('loginctl', 'echo no')
        self.command('fake-supervisorctl', '''echo "supervisorctl $*" >> "$FIXTURE/calls"
[[ "$*" == *shutdown* ]] && { touch "$2.mock-stopped"; exit 0; }
[[ "$*" == *' pid'* && -f "$2.mock-stopped" ]] && exit 1
[[ "$*" == *' start '* || "$*" == *' restart '* ]] && rm -f "$2.mock-stopped"
if [[ "$*" == *status* ]]; then
    [[ "${FAIL_SERVICE:-}" == 1 ]] && { echo 'spm FATAL'; exit 3; }
    echo 'spm RUNNING pid 123, uptime 0:00:10'
fi''')
        self.command('fake-supervisord', 'echo "supervisord $*" >> "$FIXTURE/calls"; rm -f "$2.mock-stopped"')
        self.command('python3', '''echo "python3 $*" >> "$FIXTURE/calls"
[[ "${FAIL_VENV:-}" == 1 ]] && exit 1
[[ "$1 $2" == '-m venv' ]] || exit 91
mkdir -p "$3/bin"
cp "$FIXTURE/commands/fake-pip-python" "$3/bin/python"
cp "$FIXTURE/commands/fake-supervisord" "$3/bin/supervisord"
cp "$FIXTURE/commands/fake-supervisorctl" "$3/bin/supervisorctl"''')
        self.command('fake-pip-python', 'echo "pip $*" >> "$FIXTURE/calls"')

    def run_installer(self, *args, **env):
        result = subprocess.run(['bash', str(self.script), *args],
                                env=dict(self.env, **env), stdin=subprocess.DEVNULL,
                                capture_output=True, text=True, timeout=25)
        self.output = result.stdout + result.stderr
        return result.returncode

    def role_dir(self, role='server'):
        return self.home / f'.local/share/spm/{role}'

    def installed(self, role='server'):
        return self.role_dir(role) / f'spm-{role}'

    def control(self, role, action, **env):
        return subprocess.run(['bash', str(self.home / '.local/bin/spm-user'), role, action],
                              env=dict(self.env, **env), capture_output=True, text=True, timeout=25)

    def test_default_uses_user_systemd_and_only_server(self):
        self.assertEqual(self.run_installer(), 0, self.output)
        self.assertEqual(self.installed().read_bytes(), self.payload)
        self.assertFalse(self.installed('agent').exists())
        calls = self.log.read_text()
        self.assertIn('systemctl --user restart spm-server', calls)
        self.assertNotIn('useradd', calls)
        self.assertNotIn('pip ', calls)
        self.assertIn('/releases/download/v0.1.0/spm-server-linux-amd64', calls)
        self.assertEqual((self.role_dir() / 'config.env').stat().st_mode & 0o777, 0o600)
        unit = self.home / '.config/systemd/user/spm-server.service'
        self.assertIn('WantedBy=default.target', unit.read_text())
        self.assertNotIn('User=', unit.read_text())
        for action in ('start', 'stop', 'restart', 'status'):
            self.assertEqual(self.control('server', action).returncode, 0)

    def test_missing_user_systemd_installs_isolated_supervisor_for_agent(self):
        self.assertEqual(self.run_installer('agent', '--version', 'v0.1.0',
                                            NO_USER_SYSTEMD='1', MOCK_ARCH='aarch64'), 0, self.output)
        self.assertFalse(self.installed().exists())
        self.assertEqual(self.installed('agent').read_bytes(), self.payload)
        calls = self.log.read_text()
        self.assertIn('supervisor==4.3.0', calls)
        self.assertIn('spm-agent-linux-arm64', calls)
        self.assertNotIn('systemctl --user restart', calls)
        conf = (self.role_dir('agent') / 'supervisord.conf').read_text()
        self.assertIn('[unix_http_server]', conf)
        self.assertNotIn('[inet_http_server]', conf)
        self.assertIn('chmod=0600', conf)
        self.assertIn('stdout_logfile_maxbytes=5MB', conf)
        for action in ('start', 'stop', 'restart', 'status', 'logs'):
            self.assertEqual(self.control('agent', action).returncode, 0, action)

    def test_reinstall_preserves_configuration_data_and_manager(self):
        self.assertEqual(self.run_installer(NO_USER_SYSTEMD='1'), 0, self.output)
        config = self.role_dir() / 'config.env'
        config.write_text('SPM_ADMIN_PASSWORD=literal $(touch should-not-exist)\n')
        data = self.role_dir() / 'data/spm.db'
        data.parent.mkdir(exist_ok=True)
        data.write_bytes(b'preserved database')
        self.assertEqual(self.run_installer(SPM_ADMIN_PASSWORD=''), 0, self.output)
        self.assertEqual(data.read_bytes(), b'preserved database')
        self.assertEqual(config.read_text(), 'SPM_ADMIN_PASSWORD=literal $(touch should-not-exist)\n')
        self.assertEqual((self.role_dir() / 'manager').read_text().strip(), 'supervisor')

    def test_agent_installs_without_node_id_and_blocks_legacy_downgrade(self):
        self.assertEqual(self.run_installer('agent', '--version', 'v0.1.1', SPM_NODE_ID='',
                                            NO_USER_SYSTEMD='1'), 0, self.output)
        config = self.role_dir('agent') / 'config.env'
        original = config.read_bytes()
        self.assertNotIn(b'SPM_NODE_ID', original)
        self.assertIn(b'SPM_TOKEN=fixture-token-secret', original)
        self.assertNotIn(self.env['SPM_TOKEN'], self.output)
        self.installed('agent').write_bytes(b'current agent')
        self.assertNotEqual(self.run_installer('agent', '--version', 'v0.1.0'), 0)
        self.assertIn('v0.1.0', self.output)
        self.assertEqual(self.installed('agent').read_bytes(), b'current agent')
        self.assertEqual(config.read_bytes(), original)

    def test_legacy_release_without_id_explains_upgrade_before_installing(self):
        self.assertNotEqual(self.run_installer('agent', '--version', 'v0.1.0', SPM_NODE_ID=''), 0)
        self.assertIn('v0.1.0', self.output)
        self.assertFalse(self.installed('agent').exists())

    def test_bad_download_or_checksum_never_replaces_or_starts(self):
        self.installed().parent.mkdir(parents=True)
        self.installed().write_bytes(b'previous')
        for key in ('FAIL_DOWNLOAD', 'BAD_HASH'):
            self.assertNotEqual(self.run_installer(**{key: '1'}), 0)
            self.assertEqual(self.installed().read_bytes(), b'previous')
        self.assertNotIn('restart', self.log.read_text())

    def test_failed_service_restores_previous_files(self):
        self.assertEqual(self.run_installer(), 0, self.output)
        self.installed().write_bytes(b'previous')
        unit = self.home / '.config/systemd/user/spm-server.service'
        unit.write_text('[Service]\nEnvironment=ORIGINAL=1\n')
        self.assertNotEqual(self.run_installer(FAIL_SERVICE='1'), 0)
        self.assertEqual(self.installed().read_bytes(), b'previous')
        self.assertEqual(unit.read_text(), '[Service]\nEnvironment=ORIGINAL=1\n')

    def test_file_install_failure_restores_previous_release(self):
        self.assertEqual(self.run_installer(), 0, self.output)
        self.installed().write_bytes(b'previous binary')
        unit = self.home / '.config/systemd/user/spm-server.service'
        unit.write_text('[Service]\nEnvironment=ORIGINAL=1\n')
        files = [self.installed(), unit, self.role_dir() / 'run',
                 self.home / '.local/bin/spm-user', self.role_dir() / 'config.env']
        original = {path: path.read_bytes() for path in files}
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
        for path, content in original.items():
            self.assertEqual(path.read_bytes(), content, str(path))

    def test_failed_first_install_can_retry_with_preserved_config(self):
        self.assertNotEqual(self.run_installer(FAIL_SERVICE='1'), 0)
        for path in [self.installed(), self.role_dir() / 'run',
                     self.home / '.local/bin/spm-user',
                     self.home / '.config/systemd/user/spm-server.service']:
            self.assertFalse(path.exists(), str(path))
        config = self.role_dir() / 'config.env'
        original = config.read_bytes()
        self.assertEqual(self.run_installer(SPM_ADMIN_PASSWORD=''), 0, self.output)
        self.assertEqual(config.read_bytes(), original)
        self.assertEqual(self.installed().read_bytes(), self.payload)

    def test_literal_secrets_are_loaded_without_evaluation(self):
        password = 'literal "quote" \\ $HOME $(touch ignored) % value'
        self.assertEqual(self.run_installer(SPM_ADMIN_PASSWORD=password), 0, self.output)
        self.assertNotIn(password, self.output)
        self.installed().write_text('#!/bin/bash\nprintf "%s" "$SPM_ADMIN_PASSWORD"\n')
        result = subprocess.run(['bash', str(self.role_dir() / 'run')], env=self.env,
                                capture_output=True, text=True, check=True, timeout=5)
        self.assertEqual(result.stdout, password)

    def test_daemon_does_not_inherit_installer_or_control_locks(self):
        self.command('fake-supervisorctl', '''[[ "$*" == *' pid'* ]] && exit 1
echo 'spm RUNNING pid 123, uptime 0:00:10' ''')
        self.command('fake-supervisord', '''for fd in 8 9; do
    [[ ! -e /proc/$$/fd/$fd ]] || { echo 'daemon inherited a lock descriptor' >&2; exit 77; }
done''')
        self.assertEqual(self.run_installer(NO_USER_SYSTEMD='1'), 0, self.output)

    def test_invalid_values_and_missing_venv_do_not_install(self):
        for args, env in [(('both',), {}), ((), {'MOCK_UID': '0'}),
                          ((), {'SPM_ADMIN_PASSWORD': 'short'}),
                          (('agent',), {'SPM_TOKEN': ''}),
                          (('agent',), {'SPM_SERVER': 'file:///etc/passwd'}),
                          (('--version', '../main'), {}),
                          ((), {'MOCK_ARCH': 'mips'}),
                          ((), {'NO_USER_SYSTEMD': '1', 'FAIL_VENV': '1'})]:
            with self.subTest(args=args, env=env):
                self.assertNotEqual(self.run_installer(*args, **env), 0)
                self.assertFalse(self.installed().exists())
                self.assertFalse(self.installed('agent').exists())


if __name__ == '__main__':
    unittest.main()
