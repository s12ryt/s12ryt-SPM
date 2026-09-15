"""Fresh CI user only: install public Release binaries without root privileges."""
import hashlib
import http.cookiejar
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time
import urllib.error
import urllib.request


def main():
    manager = sys.argv[1]
    assert manager in ('systemd', 'supervisor')
    assert os.geteuid() != 0, 'installation must run as an unprivileged account'
    home = Path.home()
    root = home / '.local/share/spm'
    assert not root.exists(), 'a fresh isolated user is required'
    installer = Path(__file__).resolve().parents[1] / 'install-user.sh'
    control = home / '.local/bin/spm-user'
    environment = dict(os.environ, SPM_ADMIN_USER='user-smoke',
                       SPM_ADMIN_PASSWORD='literal $HOME "quote" \\ 100% test',
                       SPM_LISTEN='127.0.0.1:18083')
    base = 'http://127.0.0.1:18083'
    client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))

    def api(path, data=None):
        request = urllib.request.Request(base + '/api/' + path,
                                         data=json.dumps(data).encode() if data is not None else None,
                                         headers={'Content-Type': 'application/json', 'X-SPM-CSRF': '1'})
        with client.open(request, timeout=3) as response:
            return json.load(response)

    def login():
        return api('login', {'username': 'user-smoke', 'password': environment['SPM_ADMIN_PASSWORD']})

    def install(*args, **extra):
        subprocess.run(['bash', str(installer), *args, '--version', 'v0.1.0'],
                       env=dict(environment, **extra), stdin=subprocess.DEVNULL, check=True, timeout=180)

    def manage(role, action, check=True):
        return subprocess.run([str(control), role, action], check=check,
                              capture_output=True, text=True, timeout=45)

    def pid(role):
        if manager == 'systemd':
            command = ['systemctl', '--user', 'show', f'spm-{role}', '-p', 'MainPID', '--value']
        else:
            command = [str(root / 'venv/bin/supervisorctl'), '-c',
                       str(root / role / 'supervisord.conf'), 'pid', 'spm']
        result = subprocess.run(command, check=True, capture_output=True, text=True, timeout=10)
        value = int(result.stdout.strip())
        assert value > 1
        assert Path(f'/proc/{value}').stat().st_uid == os.geteuid(), 'service must belong to test user'
        return value

    try:
        install()
        assert not (root / 'agent').exists(), 'default must install only Server'
        assert (root / 'server/manager').read_text().strip() == manager, 'must exercise selected manager'
        assert api('session') == {'admin': False, 'public': False}
        login()
        node = api('nodes', {'name': f'Unprivileged {manager} Agent'})
        config = (root / 'server/config.env').read_bytes()
        server_hash = hashlib.sha256((root / 'server/spm-server').read_bytes()).hexdigest()
        install('agent', SPM_SERVER=base, SPM_NODE_ID=node['id'], SPM_TOKEN=node['token'])
        assert (root / 'agent/manager').read_text().strip() == manager
        assert (root / 'server/config.env').read_bytes() == config
        assert hashlib.sha256((root / 'server/spm-server').read_bytes()).hexdigest() == server_hash
        deadline = time.monotonic() + 25
        while time.monotonic() < deadline:
            nodes = api('nodes')
            if nodes and nodes[0].get('latest') and nodes[0]['latest']['metrics']['cpu'] is not None:
                break
            time.sleep(1)
        else:
            raise AssertionError('installed non-root Agent did not send two real samples')
        assert nodes[0]['online']
        old_pid = pid('agent')
        os.kill(old_pid, signal.SIGKILL)
        deadline = time.monotonic() + 15
        while time.monotonic() < deadline:
            try:
                if pid('agent') != old_pid:
                    break
            except (AssertionError, ValueError, subprocess.CalledProcessError):
                pass
            time.sleep(1)
        else:
            raise AssertionError('process manager did not restart the crashed Agent')
        for action in ('status', 'logs', 'stop'):
            manage('server', action)
        try:
            api('session')
        except (urllib.error.URLError, TimeoutError):
            pass
        else:
            raise AssertionError('Server must stop listening after stop')
        manage('server', 'start')
        deadline = time.monotonic() + 10
        while True:
            try:
                login()
                break
            except urllib.error.URLError:
                if time.monotonic() >= deadline:
                    raise
                time.sleep(0.5)
        install(SPM_ADMIN_PASSWORD='ignored-during-update')
        assert (root / 'server/config.env').read_bytes() == config
        login()
        assert api('nodes')[0]['id'] == node['id'], 'update must retain database'
        for role in ('server', 'agent'):
            manage(role, 'status')
            pid(role)
            assert (root / role / 'config.env').stat().st_mode & 0o777 == 0o600
            if manager == 'supervisor':
                assert (root / role / 'control.sock').stat().st_mode & 0o777 == 0o600
        print(f'PASS: uid={os.geteuid()}, {manager}, Release-only install, role isolation, '
              'literal secrets, real samples, crash restart, stop/start, update preservation')
    finally:
        if control.exists():
            for role in ('agent', 'server'):
                manage(role, 'stop', check=False)
        if manager == 'systemd':
            for role in ('agent', 'server'):
                subprocess.run(['systemctl', '--user', 'disable', f'spm-{role}'],
                               check=False, timeout=15)


if __name__ == '__main__':
    main()
