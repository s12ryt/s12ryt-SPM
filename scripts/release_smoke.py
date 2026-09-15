"""Fresh Linux CI runner only: install actual public Release assets with systemd."""
import hashlib
import http.cookiejar
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time
import urllib.request


def main():
    tag = sys.argv[1]
    assert re.fullmatch(r'v\d+\.\d+\.\d+', tag), 'stable version required'
    assert os.geteuid() == 0, 'isolated runner root required'
    assert not Path('/etc/spm').exists(), 'requires a fresh runner without an existing SPM installation'
    installer = Path(__file__).resolve().parents[1] / 'install.sh'
    environment = dict(os.environ, SPM_ADMIN_USER='release-test',
                       SPM_ADMIN_PASSWORD='release $literal "quote" \\ only', SPM_LISTEN='127.0.0.1:18082')
    environment.pop('SPM_NODE_ID', None)
    environment.pop('SPM_SERVER', None)
    environment.pop('SPM_TOKEN', None)
    base = 'http://127.0.0.1:18082'
    client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))

    def api(path, data=None):
        request = urllib.request.Request(base + '/api/' + path,
                                         data=json.dumps(data).encode() if data is not None else None,
                                         headers={'Content-Type': 'application/json', 'X-SPM-CSRF': '1'})
        with client.open(request, timeout=5) as response:
            return json.load(response)

    def install(*args, latest=False, **extra):
        version_args = [] if latest else ['--version', tag]
        subprocess.run(['bash', str(installer), *args, *version_args],
                       env=dict(environment, **extra), stdin=subprocess.DEVNULL, check=True, timeout=100)

    try:
        install(latest=True)
        assert not Path('/opt/spm/spm-agent').exists(), 'default must not install Agent'
        assert not Path('/etc/spm/agent.env').exists()
        assert api('session') == {'admin': False, 'public': False}
        api('login', {'username': 'release-test', 'password': environment['SPM_ADMIN_PASSWORD']})
        node = api('nodes', {'name': 'Release installation smoke'})
        server_hash = hashlib.sha256(Path('/opt/spm/spm-server').read_bytes()).hexdigest()
        config = Path('/etc/spm/server.env').read_bytes()
        install('agent', '--server', base, '--token', node['token'])
        assert hashlib.sha256(Path('/opt/spm/spm-server').read_bytes()).hexdigest() == server_hash
        assert Path('/etc/spm/server.env').read_bytes() == config
        deadline = time.monotonic() + 25
        while time.monotonic() < deadline:
            nodes = api('nodes')
            if nodes and nodes[0].get('latest') and nodes[0]['latest']['metrics']['cpu'] is not None:
                break
            time.sleep(1)
        else:
            raise AssertionError('Release Agent did not report two real samples')
        assert nodes[0]['online'], 'installed Agent must be online'
        agent_config = Path('/etc/spm/agent.env').read_bytes()
        assert b'SPM_NODE_ID=' not in agent_config, 'new Agent installs must not require a node ID'
        install(SPM_ADMIN_PASSWORD='ignored-during-update')
        assert Path('/etc/spm/server.env').read_bytes() == config
        assert Path('/etc/spm/agent.env').read_bytes() == agent_config
        api('login', {'username': 'release-test', 'password': environment['SPM_ADMIN_PASSWORD']})
        assert api('nodes')[0]['id'] == node['id'], 'server update must preserve database'
        for role in ('server', 'agent'):
            assert Path(f'/etc/spm/{role}.env').stat().st_mode & 0o777 == 0o600
            subprocess.run(['systemctl', 'is-active', '--quiet', f'spm-{role}'], check=True, timeout=10)
        print('PASS: public Release assets, role isolation, SHA256, systemd, Agent installation using --server/--token without node ID, update preservation')
    finally:
        for role in ('agent', 'server'):
            subprocess.run(['systemctl', 'disable', '--now', f'spm-{role}'], check=False, timeout=30)


if __name__ == '__main__':
    main()
