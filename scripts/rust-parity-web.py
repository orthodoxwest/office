#!/usr/bin/env python3
"""Crawl the Go and Rust web servers with the same requests and compare them.

usage: rust-parity-web.py GO_BASE RUST_BASE DATES

Each request goes to both servers; the status, the headers a client acts on,
and the body must match after normalizing what legitimately differs: the
build stamp (a hash of each binary), the reminder feed's DTSTAMP, and the
random usage cookie. The request list covers every hour and the day page on
each sample date, calendar years, the reminder feed, the usage endpoints,
static assets and error pages; links found on
the day and hour pages are then followed one level. Run by
scripts/rust-parity-web.sh.
"""

import difflib
import http.client
import re
import sys
import urllib.parse

HOURS = ["lauds", "prime", "terce", "sext", "none", "vespers", "compline"]

COMPARED_HEADERS = [
    "Content-Type",
    "Cache-Control",
    "Location",
    "X-Robots-Tag",
    "Allow",
    "X-Content-Type-Options",
    "Set-Cookie",
]

NORMALIZE = [
    (re.compile(rb"\?v=[0-9a-f]{12}"), b"?v=VERSION"),
    (re.compile(rb'var VERSION = "[0-9a-f]{12}"'), b'var VERSION = "VERSION"'),
    (re.compile(rb"DTSTAMP:\d{8}T\d{6}Z"), b"DTSTAMP:NOW"),
]

COOKIE_ID = re.compile(r"office-usage=[0-9a-f]{32}")

BROWSER = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"


class Client:
    def __init__(self, base):
        u = urllib.parse.urlsplit(base)
        self.host, self.port = u.hostname, u.port
        self.conn = None

    def request(self, method, path, headers, body):
        for attempt in range(2):
            if self.conn is None:
                self.conn = http.client.HTTPConnection(self.host, self.port, timeout=120)
            try:
                # skip_host keeps Host identical for both servers.
                self.conn.putrequest(method, path, skip_host=True, skip_accept_encoding=True)
                self.conn.putheader("Host", "office.test")
                for k, v in headers.items():
                    self.conn.putheader(k, v)
                if body is not None:
                    self.conn.putheader("Content-Length", str(len(body)))
                self.conn.endheaders(body)
                resp = self.conn.getresponse()
                data = resp.read()
                if resp.getheader("Connection", "").lower() == "close":
                    self.conn.close()
                    self.conn = None
                return resp.status, resp.headers, data
            except (http.client.HTTPException, OSError):
                self.conn.close()
                self.conn = None
                if attempt:
                    raise
        raise AssertionError("unreachable")


def normalize(status, headers, body):
    # Redirect bodies are framework-generated, never rendered application UI.
    # Keep comparing the status and destination, not Go's HTML link wrapper.
    redirect = 300 <= status < 400
    if redirect:
        body = b""
    lines = [f"status {status}"]
    for name in COMPARED_HEADERS:
        if redirect and name == "Content-Type":
            continue
        for value in headers.get_all(name) or []:
            if name == "Set-Cookie":
                value = COOKIE_ID.sub("office-usage=ID", value)
            lines.append(f"{name}: {value}")
    for pattern, repl in NORMALIZE:
        body = pattern.sub(repl, body)
    return "\n".join(lines), body


def requests_for(dates):
    """The fixed request list: (method, path, headers, body)."""
    get = lambda path, **headers: ("GET", path, headers, None)
    reqs = []
    for i, date in enumerate(dates):
        reqs.append(get(f"/?date={date}"))
        for hour in HOURS:
            reqs.append(get(f"/{hour}/{date}"))
        if i % 16 == 0:
            reqs.append(get(f"/lauds?date={date}"))
            reqs.append(get(f"/prime/{date}?preview=martyrology"))
    # Undated pages resolve today in the browser's zone, else the host's.
    for cookie in [None, "tz=America/New_York", "tz=Pacific/Kiritimati", "tz=Etc/GMT+12", "tz=Nowhere/City", "tz=", 'tz="Asia/Tokyo"', "tz=america/new_york", "tz=US/Pacific"]:
        headers = {"Cookie": cookie} if cookie else {}
        reqs += [get("/", **headers), get("/compline", **headers), get("/reminders", **headers), get("/calendar", **headers), get("/nope", **headers)]
    reqs += [
        get("/calendar/2026"),
        get("/calendar/2027"),
        get("/calendar/2038"),
        get("/calendar/2026/"),
        get("/calendar/+2027"),
        get("/calendar?form=priest"),
        get("/calendar?form=deacon&x=1"),
        get("/calendar?form=bishop"),
        get("/calendar?form="),
        get("/calendar/0"),
        get("/calendar/10000"),
        get("/calendar/abc"),
        get("/calendar/2026/extra"),
        get("/calendar/%32%30%32%36"),
        # Dates the handlers reject or clamp.
        get("/lauds/2026-02-30"),
        get("/lauds/2026-1-01"),
        get("/lauds/2026-01-01x"),
        get("/lauds/x"),
        get("/lauds/%22%3Cb%3E"),
        get("/lauds/0000-01-01"),
        get("/lauds/9999-12-31"),
        get("/?date=0000-01-01"),
        get("/?date=9999-12-31"),
        get("/?date=bad"),
        get("/?date=2026-13-01"),
        get("/lauds?date=bad"),
        get("/lauds?date=2026-02-29"),
        get("/matins/2026-01-01"),
        get("/a/b/c"),
        get("/lauds/2026-09-06?preview=martyrology"),
        get("/prime/2026-09-06?preview=other"),
        get("/prime/2026-09-06?preview"),
        # Application paths, escaped dates, and malformed inputs.
        # Framework path matching is covered by Rust router tests.
        get("/lauds/2026%2D03%2D11"),
        get("/lauds/2026-03-11/"),
        get("/lauds%2F2026-03-11"),
        get("/lauds/2026-03-11%"),
        get("/lauds/2026-03-11%zz"),
        get("/reminders/"),
        get("/api/usage/"),
        get("/static/style.css"),
        get("/static/style.css?v=abc"),
        get("/static/missing.js"),
        get("/static/missing.js?v=abc"),
        get("/static/sw.js"),
        get("/static/manifest.webmanifest"),
        get("/static/fonts/README.txt"),
        get("/static/fonts/eb-garamond-regular.woff2?v=1"),
        get("/static/icons/icon-192.png"),
        get("/static/plaster.jpg"),
        get("/static/favicon.svg"),
        get("/sw.js"),
        # Any method reaches the page handlers; HEAD drops the body.
        ("HEAD", "/lauds/2026-03-11", {}, None),
        ("HEAD", "/calendar", {}, None),
        ("HEAD", "/static/style.css", {}, None),
        ("POST", "/reminders", {}, b""),
        ("DELETE", "/vespers/2026-03-11", {}, None),
        # The reminder feed.
        get("/office.ics"),
        get("/office.ics?lauds=06:45"),
        get("/office.ics?lauds=6:45&vespers=18:00&compline=21:00&tz=America/New_York&days=mon-fri,sun&alarm=0&horizon=366"),
        get("/office.ics?prime=07:30&tz=Europe/London&days=sat-mon&alarm=none&horizon=45", **{"X-Forwarded-Proto": "https"}),
        get("/office.ics?none=02:30&tz=America/New_York&horizon=366"),
        get("/office.ics?terce=01:30&tz=America/New_York&horizon=366&alarm=1440"),
        get("/office.ics?sext=12:00&tz=US/Eastern"),
        get("/office.ics?lauds=25:00"),
        get("/office.ics?lauds=06:5"),
        get("/office.ics?lauds=06:45&alarm=-1"),
        get("/office.ics?lauds=06:45&alarm=1441"),
        get("/office.ics?lauds=06:45&alarm=x"),
        get("/office.ics?lauds=06:45&tz=Nowhere/City"),
        get("/office.ics?lauds=06:45&tz=america/new_york"),
        get("/office.ics?lauds=06:45&horizon=0"),
        get("/office.ics?lauds=06:45&horizon=367"),
        get("/office.ics?lauds=06:45&days=funday"),
        get("/office.ics?lauds=06:45&days=mon-xyz"),
        get("/office.ics?lauds=06:45&days=mon,"),
        get("/office.ics?lauds=06:45;vespers=18:00"),
        # Usage: the beacon and the report. Both servers see the same
        # sequence, so their counts stay in step.
        get("/api/usage"),
        ("POST", "/api/usage", {}, b"lauds"),
        ("POST", "/api/usage", {"X-Office-Usage": "1", "User-Agent": BROWSER, "Sec-Fetch-Site": "cross-site"}, b"lauds"),
        ("POST", "/api/usage", {"X-Office-Usage": "1", "User-Agent": "curl/8.0"}, b"lauds"),
        ("POST", "/api/usage", {"X-Office-Usage": "1"}, b"lauds"),
        ("POST", "/api/usage", {"X-Office-Usage": "1", "User-Agent": BROWSER}, b"matins"),
        ("POST", "/api/usage", {"X-Office-Usage": "1", "User-Agent": BROWSER}, b"lauds " + b"x" * 100),
        ("POST", "/api/usage", {"X-Office-Usage": "1", "User-Agent": BROWSER, "Origin": "https://evil.test"}, b"lauds"),
        ("POST", "/api/usage", {"X-Office-Usage": "1", "User-Agent": BROWSER, "Origin": "ftp://office.test"}, b"lauds"),
        ("POST", "/api/usage", {"X-Office-Usage": "1", "User-Agent": BROWSER, "Origin": "https://office.test"}, b"lauds appearance:apse screen:mobile prayer-form:priest"),
        ("POST", "/api/usage", {"X-Office-Usage": "1", "User-Agent": BROWSER, "Cookie": "office-usage=" + "ab" * 16}, b"ordo screen:desktop"),
        ("POST", "/api/usage", {"X-Office-Usage": "1", "User-Agent": BROWSER, "Cookie": "office-usage=" + "ab" * 16}, b"vespers prayer-form:deacon"),
        ("POST", "/api/usage", {"X-Office-Usage": "1", "User-Agent": BROWSER, "X-Forwarded-Proto": "https"}, b"reminders future:token"),
        ("POST", "/api/usage", {"X-Office-Usage": "1", "User-Agent": BROWSER, "Cookie": "office-usage=zz"}, b"site"),
        get("/admin/usage"),
        get("/admin/usage?days=7"),
        get("/admin/usage?days=90"),
        get("/admin/usage?days=365"),
        get("/admin/usage?days=5"),
        get("/admin/usage?days=x"),
        ("HEAD", "/admin/usage", {}, None),
        ("POST", "/admin/usage", {}, b""),
    ]
    return reqs


LINK = re.compile(rb'(?:href|src)="(/[^"#]*)')


def links(body):
    return {m.group(1).decode().replace("&amp;", "&") for m in LINK.finditer(body)}


def main():
    go, rs = Client(sys.argv[1]), Client(sys.argv[2])
    dates = [d for d in sys.argv[3].split(",") if d]
    queue = requests_for(dates)
    seeded = len(queue)
    seen = {(m, p) for m, p, _, _ in queue}
    checked = failed = 0
    statuses = {}
    followed = set()
    i = 0
    while i < len(queue):
        method, path, headers, body = queue[i]
        i += 1
        checked += 1
        g_head, g_body = normalize(*go.request(method, path, headers, body))
        r_head, r_body = normalize(*rs.request(method, path, headers, body))
        status = g_head.split("\n")[0].removeprefix("status ")
        statuses[status] = statuses.get(status, 0) + 1
        if (g_head, g_body) != (r_head, r_body):
            failed += 1
            print(f"parity: {method} {path} {headers or ''} DIFFERS", file=sys.stderr)
            diff = difflib.unified_diff(
                (g_head + "\n" + g_body.decode(errors="replace")).splitlines(),
                (r_head + "\n" + r_body.decode(errors="replace")).splitlines(),
                "go",
                "rust",
                lineterm="",
                n=1,
            )
            for n, line in enumerate(diff):
                if n > 30:
                    break
                print(line[:300], file=sys.stderr)
            continue
        # Follow the seeded day and hour pages' links one level.
        if i <= seeded and method == "GET" and not headers and g_head.startswith("status 200") and path not in followed:
            first = path.lstrip("/").split("/")[0].split("?")[0]
            if first in HOURS or first == "":
                followed.add(path)
                for link in sorted(links(g_body)):
                    if ("GET", link) not in seen:
                        seen.add(("GET", link))
                        queue.append(("GET", link, {}, None))
    if failed:
        print(f"parity: web {failed} of {checked} responses differ", file=sys.stderr)
        sys.exit(1)
    summary = ", ".join(f"{n} × {s}" for s, n in sorted(statuses.items()))
    print(f"parity: web {checked} responses identical ({summary})")


if __name__ == "__main__":
    main()
