"""Finite local process measurement; NOT Linux container/Postgres capacity proof."""
import json, os, platform, resource, secrets, subprocess, time, urllib.request
from pathlib import Path

env = dict(os.environ)
env.update(ADMIN_USERNAME='measurement-only', ADMIN_PASSWORD=secrets.token_hex(24))
env.update(DEV_MODE='true', SERVER_ADDR='127.0.0.1:8081', PUBLIC_ORIGIN='http://127.0.0.1:8081', SMOKE_ORIGIN='http://127.0.0.1:8081', SMOKE_BASE_URL='http://127.0.0.1:8081', DATA_DIR='.cache/measure-data', DATABASE_URL='')
process = subprocess.Popen(['.cache/papergrade-dev', 'api'], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
start = time.monotonic()
try:
    deadline = start + 8
    while True:
        if process.poll() is not None:
            raise RuntimeError('API terminated during startup')
        try:
            with urllib.request.urlopen('http://127.0.0.1:8081/readyz', timeout=.3) as response:
                if response.status == 200:
                    break
        except Exception:
            if time.monotonic() > deadline:
                raise
    smoke = subprocess.run(['node', 'scripts/smoke.mjs'], env=env, text=True, capture_output=True, timeout=20)
    if smoke.returncode:
        raise RuntimeError(smoke.stderr)
    for _ in range(100):
        with urllib.request.urlopen('http://127.0.0.1:8081/readyz', timeout=1) as response:
            response.read()
finally:
    process.terminate()
    _, status, usage = os.wait4(process.pid, 0)
    process.returncode = os.waitstatus_to_exitcode(status)

rss_bytes = usage.ru_maxrss if platform.system() == 'Darwin' else usage.ru_maxrss * 1024
print(json.dumps({'platform':platform.platform(), 'mode':'compiled API, memory_dev, cloud disabled', 'exercise':'startup + live HTTP roster/publish/SVG smoke + 100 readiness requests', 'max_rss_mib':round(rss_bytes/1024**2,2), 'user_cpu_seconds':round(usage.ru_utime,3), 'system_cpu_seconds':round(usage.ru_stime,3), 'elapsed_seconds':round(time.monotonic()-start,3), 'excludes':'PostgreSQL, Worker, Caddy, Docker, real cloud, real phone; not idle steady state or 2C4G acceptance'}, ensure_ascii=False, indent=2))
