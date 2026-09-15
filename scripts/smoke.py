"""Run isolated binary integration checks; optional browser window has a hard TTL."""
import argparse
import http.cookiejar
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
import urllib.request
import urllib.error


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--serve-seconds', type=int, default=0)
    parser.add_argument('--binary-suffix', default='.exe' if os.name == 'nt' else '')
    parser.add_argument('--port', type=int, default=18080)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[1]
    suffix = args.binary_suffix
    env = os.environ.copy()
    env.pop('DATABASE_URL', None)
    env.update(SPM_ADMIN_USER='admin', SPM_ADMIN_PASSWORD='local-smoke-test-only', SPM_LISTEN=f'127.0.0.1:{args.port}')
    base = f'http://127.0.0.1:{args.port}'
    jar = http.cookiejar.CookieJar()
    client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))

    def api(path, body=None, method=None):
        request = urllib.request.Request(base + path, data=None if body is None else json.dumps(body).encode(), method=method, headers={'Content-Type': 'application/json', 'X-SPM-CSRF': '1'})
        with client.open(request, timeout=10) as response:
            return json.load(response)

    processes = []
    with tempfile.TemporaryDirectory(prefix='spm-smoke-') as data:
        env['SPM_DATA_DIR'] = data
        try:
            processes.append(subprocess.Popen([str(root / 'bin' / ('spm-server' + suffix))], env=env))
            deadline = time.monotonic() + 15
            while True:
                try:
                    assert api('/api/session') == {'admin': False, 'public': False}
                    break
                except urllib.error.URLError:
                    if time.monotonic() > deadline:
                        raise
                    time.sleep(.2)
            try:
                api('/api/nodes')
                raise AssertionError('private endpoint exposed')
            except urllib.error.HTTPError as error:
                assert error.code == 401
            api('/api/login', {'username': 'admin', 'password': 'local-smoke-test-only'})
            credential = api('/api/nodes', {'name': '本機 Windows 實測' if os.name == 'nt' else '本機 Linux 實測'})
            agent_env = env | {'SPM_SERVER': base, 'SPM_NODE_ID': credential['id'], 'SPM_TOKEN': credential['token']}
            processes.append(subprocess.Popen([str(root / 'bin' / ('spm-agent' + suffix))], env=agent_env))
            deadline = time.monotonic() + 20
            while True:
                nodes = api('/api/nodes')
                if nodes and nodes[0]['latest'] and nodes[0]['latest']['metrics']['cpu'] is not None:
                    break
                assert time.monotonic() < deadline, 'agent did not report two real samples'
                time.sleep(.3)
            assert nodes[0]['online']
            now = int(time.time() * 1000) + 1
            points = api(f'/api/nodes/{credential["id"]}/history?from={now-60000}&to={now}&limit=60')
            assert len(points) >= 2
            assert credential['token'] not in json.dumps(nodes)
            settings = api('/api/settings')['settings']
            settings['public'] = True
            api('/api/settings', settings, 'PUT')
            print('PASS: private access, login, real Agent upload, CPU rate, history, token redaction, settings', flush=True)
            if args.serve_seconds:
                print(f'Browser fixture available at {base}; automatic cleanup in {min(args.serve_seconds, 900)} seconds', flush=True)
                time.sleep(min(args.serve_seconds, 900))
        finally:
            for process in reversed(processes):
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=5)


if __name__ == '__main__':
    main()
