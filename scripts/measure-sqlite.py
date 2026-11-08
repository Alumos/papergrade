"""Measure the real resident memory of the SQLite deployment processes.

Local process measurement only; not a Linux container or 2C4G capacity proof.
"""
import json, os, platform, secrets, subprocess, tempfile, time, urllib.request

def to_mib(usage):
    return usage.ru_maxrss if platform.system() == 'Darwin' else usage.ru_maxrss * 1024

def run(role, exercise):
    tmp = tempfile.mkdtemp(prefix='pg-measure-')
    env = dict(os.environ)
    env.update(DATA_DIR=tmp, SQLITE_PATH=os.path.join(tmp, 'papergrade.db'),
               DEV_MODE='false', APP_ENCRYPTION_KEY=secrets.token_hex(32),
               ADMIN_USERNAME='measurement-only', ADMIN_PASSWORD=secrets.token_hex(24),
               PUBLIC_ORIGIN='https://measurement.invalid', GOMAXPROCS='1')
    if role == 'api':
        env.update(SERVER_ADDR='127.0.0.1:8081')
    start = time.monotonic()
    process = subprocess.Popen(['.cache/papergrade-dev', role], env=env,
                               stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    usage = None
    try:
        if role == 'api':
            deadline = start + 10
            while True:
                if process.poll() is not None:
                    raise RuntimeError('API terminated during startup')
                try:
                    with urllib.request.urlopen('http://127.0.0.1:8081/readyz', timeout=.3) as r:
                        if r.status == 200:
                            break
                except Exception:
                    if time.monotonic() > deadline:
                        raise
            for _ in range(100):
                with urllib.request.urlopen('http://127.0.0.1:8081/readyz', timeout=1) as r:
                    r.read()
        else:
            time.sleep(3)
    finally:
        if process.poll() is None:
            process.terminate()
        try:
            _, status, usage = os.wait4(process.pid, 0)
        except ChildProcessError:
            usage = None
    if usage is None:
        raise RuntimeError('could not collect rusage for ' + role)
    return {
        'role': role,
        'max_rss_mib': round(to_mib(usage) / 1024**2, 2),
        'cpu_seconds': round(usage.ru_utime + usage.ru_stime, 3),
        'elapsed_seconds': round(time.monotonic() - start, 2),
        'exercise': exercise,
    }

results = [
    run('api', 'startup + 100 readiness requests, SQLite file'),
    run('worker', 'idle poll loop on an empty queue, SQLite file'),
]
print(json.dumps({'platform': platform.platform(), 'results': results,
                  'excludes': 'gateway/Caddy, Docker overhead, real OCR/AI calls, phone capture'},
                 ensure_ascii=False, indent=2))
