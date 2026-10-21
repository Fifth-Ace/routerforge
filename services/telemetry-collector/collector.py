"""RouterForge Collector: protocol v1/v2, installation accounting and module statistics.

No IP, URL, event payload or installation ID is logged. Run behind a private
HTTPS reverse proxy; only /v1/events should be publicly routed.
"""
import base64
import contextlib
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
CHANNELS = frozenset(("stable", "beta", "dev"))
MODULES = frozenset(("admin", "monitoring", "dns", "network-tools", "nfqws-manager",
                     "antiscan-manager", "port-access-manager", "crowdsec-manager", "profiling"))
# Legacy v1 remains accepted for devices that have not yet upgraded.
BANDS = frozenset(("0-19", "20-39", "40-59", "60-79", "80-100", "unavailable"))
TEMP_BANDS = frozenset(("below-40", "40-49", "50-59", "60-69", "70-79", "80-plus", "unavailable"))
HEALTH = frozenset(("ok", "degraded", "unavailable"))
EVENT_KEYS = {
    1: {
        "register": frozenset(("schema_version", "kind", "installation_id", "event_id", "core_version")),
        "presence": frozenset(("schema_version", "kind", "installation_id", "event_id")),
        "diagnostic": frozenset(("schema_version", "kind", "installation_id", "event_id", "core_version", "architecture", "cpu_load_band", "ram_load_band", "temperature_band_c", "health_code")),
    },
    2: {
        "register": frozenset(("schema_version", "kind", "installation_id", "event_id", "core_version", "release_channel")),
        "presence": frozenset(("schema_version", "kind", "installation_id", "event_id", "core_version", "release_channel")),
        "diagnostic": frozenset(("schema_version", "kind", "installation_id", "event_id", "core_version", "release_channel", "architecture", "modules")),
    },
}


def validate(payload):
    if not isinstance(payload, dict) or type(payload.get("schema_version")) is not int or payload["schema_version"] not in EVENT_KEYS:
        raise ValueError("unsupported schema")
    schema = payload["schema_version"]
    kind = payload.get("kind")
    if kind not in EVENT_KEYS[schema] or set(payload) != EVENT_KEYS[schema][kind]:
        raise ValueError("unexpected event fields")
    if not isinstance(payload.get("installation_id"), str) or not HEX32.fullmatch(payload["installation_id"]):
        raise ValueError("invalid installation_id")
    if not isinstance(payload.get("event_id"), str) or not HEX32.fullmatch(payload["event_id"]):
        raise ValueError("invalid event_id")
    if schema == 2 or kind in ("register", "diagnostic"):
        if not isinstance(payload.get("core_version"), str) or not VERSION.fullmatch(payload["core_version"]):
            raise ValueError("invalid core version")
    if schema == 2:
        if payload.get("release_channel") not in CHANNELS:
            raise ValueError("invalid release channel")
        if kind == "diagnostic":
            if payload.get("architecture") not in ARCHES:
                raise ValueError("invalid architecture")
            mods = payload.get("modules")
            if not isinstance(mods, list) or len(mods) > len(MODULES):
                raise ValueError("invalid module list")
            seen = set()
            for mod in mods:
                if not isinstance(mod, dict) or set(mod) != {"id", "version"}:
                    raise ValueError("invalid module record")
                name, version = mod["id"], mod["version"]
                if not isinstance(name, str) or name not in MODULES or name in seen:
                    raise ValueError("unknown or repeated module")
                if not isinstance(version, str) or not VERSION.fullmatch(version):
                    raise ValueError("invalid module version")
                seen.add(name)
    elif kind == "diagnostic":
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

    @contextlib.contextmanager
    def connect(self):
        db = sqlite3.connect(self.path, timeout=5, isolation_level=None)
        try:
            db.execute("PRAGMA busy_timeout=5000")
            db.execute("PRAGMA journal_mode=WAL")
            yield db
        finally:
            db.close()

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
                day TEXT NOT NULL, architecture TEXT NOT NULL, health_code TEXT NOT NULL,
                core_version TEXT NOT NULL, cpu_band TEXT NOT NULL, ram_band TEXT NOT NULL,
                temp_band TEXT NOT NULL, count INTEGER NOT NULL,
                PRIMARY KEY(day,architecture,health_code,core_version,cpu_band,ram_band,temp_band)
            );
            CREATE TABLE IF NOT EXISTS installed_modules (
                installation_id TEXT NOT NULL,
                module_id TEXT NOT NULL,
                version TEXT NOT NULL,
                PRIMARY KEY(installation_id,module_id),
                FOREIGN KEY(installation_id) REFERENCES installations(installation_id)
            );
            CREATE INDEX IF NOT EXISTS idx_installations_last_seen ON installations(last_seen);
            """)
            cols = {row[1] for row in db.execute("PRAGMA table_info(installations)")}
            for name, definition in (
                ("current_core_version", "TEXT NOT NULL DEFAULT ''"),
                ("release_channel", "TEXT NOT NULL DEFAULT 'unknown'"),
                ("architecture", "TEXT NOT NULL DEFAULT 'unknown'"),
                ("last_diagnostic_at", "TEXT NOT NULL DEFAULT ''"),
            ):
                if name not in cols:
                    db.execute("ALTER TABLE installations ADD COLUMN " + name + " " + definition)
            db.execute("UPDATE installations SET current_core_version=first_core_version WHERE current_core_version=''")

    def receive(self, payload, now=None):
        validate(payload)
        now = now or dt.datetime.now(dt.timezone.utc)
        stamp = now.isoformat(timespec="seconds")
        kind = payload["kind"]
        schema = payload["schema_version"]
        with self.connect() as db:
            try:
                db.execute("BEGIN IMMEDIATE")
                if db.execute("SELECT 1 FROM events WHERE event_id=?", (payload["event_id"],)).fetchone():
                    db.commit()
                    return True
                iid = payload["installation_id"]
                exists = db.execute("SELECT 1 FROM installations WHERE installation_id=?", (iid,)).fetchone()
                if not exists and kind != "register":
                    db.rollback()
                    return False
                if kind == "register":
                    db.execute("""INSERT INTO installations(installation_id,first_seen,last_seen,first_core_version,current_core_version,release_channel)
                                  VALUES (?,?,?,?,?,?) ON CONFLICT(installation_id)
                                  DO UPDATE SET last_seen=excluded.last_seen""",
                               (iid, stamp, stamp, payload["core_version"], payload["core_version"],
                                payload.get("release_channel", "unknown")))
                else:
                    db.execute("UPDATE installations SET last_seen=? WHERE installation_id=?", (stamp, iid))
                if schema == 2:
                    db.execute("""UPDATE installations SET current_core_version=?,release_channel=?
                                  WHERE installation_id=?""", (payload["core_version"], payload["release_channel"], iid))
                    if kind == "diagnostic":
                        db.execute("UPDATE installations SET architecture=?, last_diagnostic_at=? WHERE installation_id=?",
                                   (payload["architecture"], stamp, iid))
                        # Authoritative snapshot: removed modules must disappear from counts.
                        db.execute("DELETE FROM installed_modules WHERE installation_id=?", (iid,))
                        db.executemany("INSERT INTO installed_modules(installation_id,module_id,version) VALUES (?,?,?)",
                                       [(iid, x["id"], x["version"]) for x in payload["modules"]])
                elif kind == "diagnostic":
                    dims = (now.date().isoformat(), payload["architecture"], payload["health_code"],
                            payload["core_version"], payload["cpu_load_band"], payload["ram_load_band"],
                            payload["temperature_band_c"])
                    db.execute("""INSERT INTO diagnostics(day,architecture,health_code,core_version,cpu_band,ram_band,temp_band,count)
                                  VALUES (?,?,?,?,?,?,?,1)
                                  ON CONFLICT(day,architecture,health_code,core_version,cpu_band,ram_band,temp_band)
                                  DO UPDATE SET count=count+1""", dims)
                db.execute("INSERT INTO events(event_id,kind,received_at) VALUES(?,?,?)",
                           (payload["event_id"], kind, stamp))
                db.commit()
                return True
            except Exception:
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
            channels = db.execute("""SELECT release_channel, COUNT(*) FROM installations
                                     WHERE last_seen>=? GROUP BY release_channel ORDER BY COUNT(*) DESC""", (d30,)).fetchall()
            versions = db.execute("""SELECT release_channel,current_core_version,COUNT(*) FROM installations
                                     WHERE last_seen>=? GROUP BY release_channel,current_core_version
                                     ORDER BY COUNT(*) DESC LIMIT 15""", (d30,)).fetchall()
            coverage = db.execute("SELECT COUNT(*) FROM installations WHERE last_diagnostic_at>=?", (d7,)).fetchone()[0]
            module_counts = db.execute("""SELECT m.module_id,COUNT(*) FROM installed_modules m
                    JOIN installations i ON i.installation_id=m.installation_id
                    WHERE i.last_seen>=? AND i.last_diagnostic_at>=?
                    GROUP BY m.module_id ORDER BY COUNT(*) DESC,m.module_id""", (d30,d7)).fetchall()
            module_versions = db.execute("""SELECT m.module_id,m.version,COUNT(*) FROM installed_modules m
                    JOIN installations i ON i.installation_id=m.installation_id
                    WHERE i.last_seen>=? AND i.last_diagnostic_at>=?
                    GROUP BY m.module_id,m.version ORDER BY m.module_id,COUNT(*) DESC LIMIT 60""", (d30,d7)).fetchall()
            architectures = db.execute("""SELECT architecture, COUNT(*) FROM installations
                            WHERE last_seen>=? AND last_diagnostic_at>=?
                            GROUP BY architecture ORDER BY COUNT(*) DESC""", (d30,d7)).fetchall()
            legacy_arch = db.execute("SELECT architecture,SUM(count) FROM diagnostics GROUP BY architecture ORDER BY 2 DESC").fetchall()
        return {"registered_total": row[0], "new_24h": row[1], "active_7d": row[2],
                "active_30d": row[3], "daily": daily, "channels": channels, "versions": versions,
                "telemetry_7d": coverage, "module_stats": module_counts,
                "module_versions": module_versions, "architectures": architectures,
                "architecture_reports": legacy_arch}

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
        return  # No IP, payloads or identifiers in application logs

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
        body = dashboard(m).encode("utf-8")
        self._headers(200, "text/html; charset=utf-8")
        self.wfile.write(body)


def dashboard(m):
    def number(v):
        return f"{int(v):,}".replace(",", " ")
    def card(name, value, note=""):
        return (f'<div class="card"><div class="caption">{html.escape(name)}</div>'
                f'<div class="value">{number(value)}</div><div class="muted">{html.escape(note)}</div></div>')
    def row(*items):
        return "<tr>" + "".join(f"<td>{html.escape(str(i))}</td>" for i in items) + "</tr>"
    def table(headers, values, empty="Пока нет данных"):
        body = "".join(row(*v) for v in values) if values else f'<tr><td colspan="{len(headers)}" class="muted">{html.escape(empty)}</td></tr>'
        return '<div class="table-wrap"><table><thead>' + row(*headers) + '</thead><tbody>' + body + '</tbody></table></div>'
    def bars(data, denominator):
        if not data:
            return '<p class="muted">Нет свежих сведений.</p>'
        lines = []
        for name, count in data:
            width = round(100 * count / max(1,denominator))
            lines.append('<div class="bar-line"><div class="bar-label">'
                         + html.escape(str(name)) + '<span>' + number(count) + '</span></div>'
                         + f'<div class="bar-bg"><div class="bar-fill" style="width:{min(100,width)}%"></div></div></div>')
        return "".join(lines)
    modules = [(name, n) for name, n in m["module_stats"]]
    versions = [(f"{c.upper()} · {v}", number(n)) for c,v,n in m["versions"]]
    modversions = [(name, ver, number(n)) for name,ver,n in m["module_versions"]]
    return f'''<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>RouterForge — статистика проекта</title>
<style>
:root{{color-scheme:dark;font-family:system-ui,-apple-system,Segoe UI,sans-serif;background:#0d1218;color:#eaf0f6}}
*{{box-sizing:border-box}}body{{margin:0;padding:35px 18px}}main{{max-width:1120px;margin:auto}}header{{display:flex;align-items:flex-start;justify-content:space-between;gap:15px;flex-wrap:wrap;margin-bottom:28px}}
h1{{font-size:26px;margin:0 0 8px}}h2{{font-size:19px;margin:0 0 15px}}p{{line-height:1.5}}.muted,.caption{{color:#a0afbf;font-size:13px}}.caption{{font-weight:600}}.eyebrow{{color:#75b7d6;font-size:12px;letter-spacing:.1em;text-transform:uppercase;margin-bottom:6px}}
.grid{{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:13px}}.card{{padding:18px;background:#18212c;border:1px solid #334253;border-radius:12px;min-height:119px}}.value{{font-size:29px;font-weight:730;margin:8px 0}}section{{margin:25px 0}}.panel{{border:1px solid #2b3948;border-radius:12px;background:#131b24;padding:20px;margin:16px 0}}.two{{display:grid;grid-template-columns:1fr 1fr;gap:14px}}
.table-wrap{{overflow:auto}}table{{width:100%;border-collapse:collapse;font-size:13px}}td{{padding:11px 9px;border-bottom:1px solid #2d3a49;text-align:left}}tr:last-child td{{border-bottom:0}}thead td{{color:#b8cce1;font-weight:700}}.bar-line{{margin:12px 0}}.bar-label{{display:flex;justify-content:space-between;gap:8px;font-size:13px;margin-bottom:6px}}.bar-bg{{height:9px;background:#2d3b4a;border-radius:20px;overflow:hidden}}.bar-fill{{height:100%;background:#5fa6cd;border-radius:20px}}
@media(max-width:780px){{.grid{{grid-template-columns:repeat(2,minmax(0,1fr))}}.two{{grid-template-columns:1fr}}}}@media(max-width:420px){{.grid{{grid-template-columns:1fr}}}}
</style></head><body><main>
<header><div><div class="eyebrow">RouterForge / Analytics</div><h1>Статистика проекта</h1><p class="muted">Установки, каналы Stable / Beta / Dev и популярность модулей.</p></div><div class="muted">Обновляется при открытии страницы · UTC</div></header>
<section><h2>Установки RouterForge</h2><div class="grid">{card('Всего зарегистрировано',m['registered_total'])}{card('Новых за 24 часа',m['new_24h'])}{card('Активны за 7 дней',m['active_7d'])}{card('Активны за 30 дней',m['active_30d'])}</div></section>
<section class="two"><div class="panel"><h2>Каналы · активны 30 дней</h2>{bars(m['channels'],m['active_30d'])}</div><div class="panel"><h2>Новые установки по дням</h2>{table(['Дата (UTC)','Установок'],[(d,number(n)) for d,n in m['daily']])}</div></section>
<section><h2>Версии Core</h2><div class="panel">{table(['Канал','Версия','Установок'],[(c.upper(),v,number(n)) for c,v,n in m['versions']])}</div></section>
<section><h2>Модули RouterForge</h2><p class="muted">Уникальные установки с включённой телеметрией за 7 дней: {number(m['telemetry_7d'])}. При выключенной телеметрии список модулей не передаётся.</p><div class="two"><div class="panel"><h2>Популярность модулей</h2>{bars(modules,m['telemetry_7d'])}</div><div class="panel"><h2>Архитектуры</h2>{bars(m['architectures'],m['telemetry_7d'])}</div></div><div class="panel"><h2>Версии модулей</h2>{table(['Модуль','Версия','Установок'],modversions)}</div></section>
<footer class="muted">Счётчики установки и активности не означают количество устройств, включённых прямо сейчас. Нет отслеживания удаления, IP-адресов или истории посещений.</footer>
</main></body></html>'''


def run():
    password = os.environ.get("RF_ADMIN_PASSWORD", "")
    if len(password) < 20 or password in ("change-me", "changeme"):
        raise SystemExit("RF_ADMIN_PASSWORD must contain >=20 characters")
    Handler.store = Store(os.environ.get("RF_DB_PATH", "/data/telemetry.sqlite"))
    Handler.admin_password = password
    port = int(os.environ.get("RF_PORT", "8000"))
    httpd = ThreadingHTTPServer(("0.0.0.0", port), Handler)
    httpd.daemon_threads = True
    httpd.serve_forever()


if __name__ == "__main__":
    run()
