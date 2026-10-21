"""RouterForge privacy-minimal installation accounting and diagnostics collector.

Python stdlib only; runs behind an HTTPS reverse proxy. Deliberately does not log
client IPs, identifiers or inbound payloads. Never bind directly to the internet.
"""
import base64
import datetime as dt
import hmac
import html
import json
import os
import re
import sqlite3
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlsplit

MAX_BODY = 2048
HEX32 = re.compile(r"^[0-9a-f]{32}$")
VERSION = re.compile(r"^[0-9A-Za-z][0-9A-Za-z.+_~\-]{0,49}$")
ARCHES = frozenset(("mips", "mipsel", "arm64", "unknown"))
BANDS = frozenset(("0-19", "20-39", "40-59", "60-79", "80-100", "unavailable"))
TEMP_BANDS = frozenset(("below-40", "40-49", "50-59", "60-69", "70-79", "80-plus", "unavailable"))
HEALTH = frozenset(("ok", "degraded", "unavailable"))
EVENT_KEYS = {
    "register": frozenset(("schema_version", "kind", "installation_id", "event_id", "core_version")),
    "presence": frozenset(("schema_version", "kind", "installation_id", "event_id")),
    "diagnostic": frozenset(("schema_version", "kind", "installation_id", "event_id", "core_version", "architecture", "cpu_load_band", "ram_load_band", "temperature_band_c", "health_code")),
}


def validate(payload):
    if not isinstance(payload, dict) or type(payload.get("schema_version")) is not int or payload["schema_version"] != 1:
        raise ValueError("unsupported schema")
    kind = payload.get("kind")
    if kind not in EVENT_KEYS or set(payload) != EVENT_KEYS[kind]:
        raise ValueError("unexpected event fields")
    if not isinstance(payload.get("installation_id"), str) or not HEX32.fullmatch(payload["installation_id"]):
        raise ValueError("invalid installation_id")
    if not isinstance(payload.get("event_id"), str) or not HEX32.fullmatch(payload["event_id"]):
        raise ValueError("invalid event_id")
    if kind in ("register", "diagnostic"):
        if not isinstance(payload.get("core_version"), str) or not VERSION.fullmatch(payload["core_version"]):
            raise ValueError("invalid version")
    if kind == "diagnostic":
        if payload["architecture"] not in ARCHES:
            raise ValueError("invalid arch")
        if payload["cpu_load_band"] not in BANDS or payload["ram_load_band"] not in BANDS:
            raise ValueError("invalid utilization")
        if payload["temperature_band_c"] not in TEMP_BANDS or payload["health_code"] not in HEALTH:
            raise ValueError("invalid diagnosis")
    return payload


class Store:
    def __init__(self, path):
        self.path = str(path)
        Path(self.path).parent.mkdir(parents=True, exist_ok=True)
        self._setup()

    def connect(self):
        db = sqlite3.connect(self.path, timeout=5, isolation_level=None)
        db.execute("PRAGMA busy_timeout=5000")
        db.execute("PRAGMA journal_mode=WAL")
        return db

    def _setup(self):
        with self.connect() as db:
            db.executescript("""
            CREATE TABLE IF NOT EXISTS installations (
                installation_id TEXT PRIMARY KEY,
                first_seen TEXT NOT NULL,
                last_seen TEXT NOT NULL,
                first_core_version TEXT NOT NULL
            );
            CREATE TABLE IF NOT EXISTS events (
                event_id TEXT PRIMARY KEY,
                kind TEXT NOT NULL,
                received_at TEXT NOT NULL
            );
            CREATE TABLE IF NOT EXISTS diagnostics (
                day TEXT NOT NULL,
                architecture TEXT NOT NULL,
                health_code TEXT NOT NULL,
                core_version TEXT NOT NULL,
                cpu_band TEXT NOT NULL,
                ram_band TEXT NOT NULL,
                temp_band TEXT NOT NULL,
                count INTEGER NOT NULL,
                PRIMARY KEY(day,architecture,health_code,core_version,cpu_band,ram_band,temp_band)
            );
            """)

    def receive(self, payload, now=None):
        validate(payload)
        now = now or dt.datetime.now(dt.timezone.utc)
        stamp = now.isoformat(timespec="seconds")
        day = now.date().isoformat()
        kind = payload["kind"]
        with self.connect() as db:
            try:
                db.execute("BEGIN IMMEDIATE")
                old_event = db.execute("SELECT 1 FROM events WHERE event_id=?", (payload["event_id"],)).fetchone()
                if old_event:
                    db.commit()
                    return True
                old_install = db.execute("SELECT 1 FROM installations WHERE installation_id=?", (payload["installation_id"],)).fetchone()
                if not old_install and kind != "register":
                    db.rollback()
                    return False
                if kind == "register":
                    db.execute("""INSERT INTO installations(installation_id,first_seen,last_seen,first_core_version)
                                  VALUES (?,?,?,?) ON CONFLICT(installation_id)
                                  DO UPDATE SET last_seen=excluded.last_seen""",
                               (payload["installation_id"], stamp, stamp, payload["core_version"]))
                else:
                    db.execute("UPDATE installations SET last_seen=? WHERE installation_id=?", (stamp, payload["installation_id"]))
                if kind == "diagnostic":
                    dims = (day, payload["architecture"], payload["health_code"], payload["core_version"], payload["cpu_load_band"], payload["ram_load_band"], payload["temperature_band_c"])
                    db.execute("""INSERT INTO diagnostics(day,architecture,health_code,core_version,cpu_band,ram_band,temp_band,count)
                                  VALUES (?,?,?,?,?,?,?,1)
                                  ON CONFLICT(day,architecture,health_code,core_version,cpu_band,ram_band,temp_band)
                                  DO UPDATE SET count=count+1""", dims)
                db.execute("INSERT INTO events(event_id,kind,received_at) VALUES(?,?,?)", (payload["event_id"], kind, stamp))
                db.commit()
                return True
            except BaseException:
                db.rollback()
                raise

    def metrics(self, now=None):
        now = now or dt.datetime.now(dt.timezone.utc)
        d7 = (now - dt.timedelta(days=7)).isoformat(timespec="seconds")
        d30 = (now - dt.timedelta(days=30)).isoformat(timespec="seconds")
        d1 = (now - dt.timedelta(days=1)).isoformat(timespec="seconds")
        with self.connect() as db:
            row = db.execute("""SELECT COUNT(*),
                 COALESCE(SUM(CASE WHEN first_seen>=? THEN 1 ELSE 0 END),0),
                 COALESCE(SUM(CASE WHEN last_seen>=? THEN 1 ELSE 0 END),0),
                 COALESCE(SUM(CASE WHEN last_seen>=? THEN 1 ELSE 0 END),0)
                 FROM installations""", (d1, d7, d30)).fetchone()
            daily = db.execute("SELECT substr(first_seen,1,10),COUNT(*) FROM installations GROUP BY substr(first_seen,1,10) ORDER BY 1 DESC LIMIT 30").fetchall()
            arch = db.execute("SELECT architecture,SUM(count) FROM diagnostics GROUP BY architecture ORDER BY 2 DESC").fetchall()
        return {"registered_total": row[0], "new_24h": row[1], "active_7d": row[2], "active_30d": row[3], "daily": daily, "architecture_reports": arch}

    def prune_events(self, older_days=45):
        threshold = (dt.datetime.now(dt.timezone.utc) - dt.timedelta(days=older_days)).isoformat(timespec="seconds")
        with self.connect() as db:
            db.execute("DELETE FROM events WHERE received_at < ?", (threshold,))


class Handler(BaseHTTPRequestHandler):
    store = None
    admin_password = ""
    admin_user = "admin"
    server_version = "RouterForgeTelemetry"
    sys_version = ""

    def log_message(self, fmt, *args):
        # Suppress all standard request logs (IP, URL, identifiers).
        return

    def _headers(self, status, content_type):
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Cache-Control", "no-store")
        self.send_header("X-Content-Type-Options", "nosniff")
        self.send_header("Referrer-Policy", "no-referrer")
        self.send_header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'none'; frame-ancestors 'none'")
        self.end_headers()

    def json_reply(self, status, value):
        data = json.dumps(value, separators=(",", ":")).encode()
        self._headers(status, "application/json; charset=utf-8")
        self.wfile.write(data)

    def _admin_ok(self):
        raw = self.headers.get("Authorization", "")
        if not raw.startswith("Basic "):
            return False
        try:
            creds = base64.b64decode(raw[6:], validate=True).decode("utf-8")
        except (ValueError, UnicodeDecodeError):
            return False
        expected = self.admin_user + ":" + self.admin_password
        return hmac.compare_digest(creds.encode(), expected.encode())

    def do_POST(self):
        if urlsplit(self.path).path != "/v1/events" or "?" in self.path:
            return self.json_reply(404, {"error": "not found"})
        if self.headers.get("Content-Type", "").split(";")[0].strip() != "application/json":
            return self.json_reply(415, {"error": "json required"})
        try:
            size = int(self.headers.get("Content-Length", "-1"))
        except ValueError:
            size = -1
        if size < 1 or size > MAX_BODY:
            return self.json_reply(413, {"error": "invalid size"})
        try:
            data = self.rfile.read(size)
            item = json.loads(data)
            ok = self.store.receive(item)
        except (json.JSONDecodeError, ValueError, TypeError, KeyError):
            return self.json_reply(400, {"error": "invalid event"})
        except sqlite3.Error:
            return self.json_reply(503, {"error": "storage unavailable"})
        if not ok:
            return self.json_reply(409, {"error": "register required"})
        return self.json_reply(200, {"accepted": True})

    def do_GET(self):
        p = urlsplit(self.path).path
        if p == "/healthz":
            return self.json_reply(200, {"ok": True})
        if p not in ("/", "/admin"):
            return self.json_reply(404, {"error": "not found"})
        if not self._admin_ok():
            self.send_response(401)
            self.send_header("WWW-Authenticate", 'Basic realm="RouterForge Telemetry"')
            self.send_header("Cache-Control", "no-store")
            self.end_headers()
            return
        m = self.store.metrics()
        def panel(label, val):
            return f'<section class="metric"><small>{html.escape(label)}</small><strong>{int(val):,}</strong></section>'
        daily = ''.join(f'<tr><td>{html.escape(day)}</td><td>{int(num):,}</td></tr>' for day,num in m['daily'])
        arch = ''.join(f'<tr><td>{html.escape(a)}</td><td>{int(n):,}</td></tr>' for a,n in m['architecture_reports'])
        body = f'''<!doctype html><html lang="ru"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>RouterForge — статистика установок</title>
<style>body{{font:16px system-ui,sans-serif;background:#11151a;color:#e6edf3;margin:0 auto;max-width:1000px;padding:32px}}h1{{font-size:27px}}h2{{font-size:18px;margin-top:30px}}.grid{{display:grid;grid-template-columns:repeat(auto-fit,minmax(175px,1fr));gap:12px}}.metric{{background:#1b222b;border:1px solid #36404b;padding:19px;border-radius:12px;display:grid;gap:8px}}small{{color:#a8b2be}}strong{{font-size:30px}}table{{width:100%;border-collapse:collapse}}th,td{{text-align:left;border-bottom:1px solid #36404b;padding:10px}}p{{color:#a8b2be;line-height:1.5}}</style>
<h1>RouterForge · статистика установок</h1><p>Закрытая панель. Считаются уникальные регистрации, а не скачивания IPK. Последняя активность не доказывает, что устройство включено сейчас.</p>
<div class="grid">{panel('Всего зарегистрировано',m['registered_total'])}{panel('Новых за 24 часа',m['new_24h'])}{panel('Активных за 7 дней',m['active_7d'])}{panel('Активных за 30 дней',m['active_30d'])}</div>
<h2>Регистрации за дни</h2><table><thead><tr><th>Дата UTC</th><th>Регистрации</th></tr></thead><tbody>{daily}</tbody></table>
<h2>Архитектуры (число диагностических отчётов, не установок)</h2><table><thead><tr><th>Архитектура</th><th>Отчёты</th></tr></thead><tbody>{arch}</tbody></table>
</html>'''.encode("utf-8")
        self._headers(200, "text/html; charset=utf-8")
        self.wfile.write(body)


def run():
    password = os.environ.get("RF_ADMIN_PASSWORD", "")
    if len(password) < 20 or password in ("change-me", "changeme"):
        raise SystemExit("RF_ADMIN_PASSWORD must contain >=20 characters")
    path = os.environ.get("RF_DB_PATH", "/data/telemetry.sqlite")
    Handler.store = Store(path)
    Handler.admin_password = password
    port = int(os.environ.get("RF_PORT", "8000"))
    httpd = ThreadingHTTPServer(("0.0.0.0", port), Handler)
    httpd.daemon_threads = True
    httpd.serve_forever()


if __name__ == "__main__":
    run()
