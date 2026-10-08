#!/usr/bin/env python3
"""Fills a demo DockLite (scripts/demo.sh) with fake users, sites and databases.

Only reserved example domains are used (example.com/.net/.org), so nothing here can
point at a real site. Safe to run again: anything that already exists is skipped.
"""
import json, os, secrets, subprocess, sys, urllib.request, urllib.error

API, TOKEN, SITES = os.environ["DEMO_API"], os.environ["DEMO_TOKEN"], os.environ["DEMO_SITES"]


def call(method, path, body=None):
    req = urllib.request.Request(API + path, method=method, data=json.dumps(body).encode() if body is not None else None,
                                 headers={"Authorization": "Bearer " + TOKEN, "Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=300) as r:
            raw = r.read()
            return r.status, (json.loads(raw) if raw else {})
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            return e.code, json.loads(raw)
        except Exception:
            return e.code, {"error": raw.decode(errors="replace")}


def page(domain, blurb, color):
    return f"""<!doctype html><meta charset=utf-8><meta name=viewport content="width=device-width,initial-scale=1">
<title>{domain}</title><style>body{{margin:0;min-height:100vh;display:grid;place-items:center;font-family:system-ui,sans-serif;
background:linear-gradient(135deg,{color},#1b1030);color:#fff;text-align:center}}h1{{font-size:2.2rem;margin:.2em}}p{{opacity:.8}}</style>
<div><h1>{domain}</h1><p>{blurb}</p></div>"""


# connect the pretend Cloudflare and import its example.* domains
call("POST", "/api/dns/config", {"api_token": "demo-token", "enabled": True})
call("POST", "/api/dns/zones/import")

# demo users (passwords are random and never shown; log in as the demo admin instead)
_, listing = call("GET", "/api/users")
users = {u["username"]: u["id"] for u in listing.get("users", [])}
for name in ("alice", "bob"):
    if name not in users:
        call("POST", "/api/users", {"username": name, "password": secrets.token_urlsafe(18), "isAdmin": False})
_, listing = call("GET", "/api/users")
users = {u["username"]: u["id"] for u in listing.get("users", [])}
users["demo"] = users.get("demo")

SITE_PLAN = [  # owner, domain, type, port, running, blurb, colour
    ("alice", "blog.example.com", "static", None, True, "A small personal blog", "#7b2ff7"),
    ("alice", "shop.example.com", "php", None, True, "Handmade goods, shipped worldwide", "#e83e8c"),
    ("bob", "portfolio.example.net", "static", None, True, "Photography and design", "#0ea5e9"),
    ("bob", "api.example.net", "node", 3000, True, "A tiny JSON API", "#16a34a"),
    ("demo", "docs.example.org", "static", None, True, "Project documentation", "#f59e0b"),
    ("demo", "status.example.org", "static", None, False, "Status page (stopped)", "#64748b"),
]


def container_id(domain):
    out = subprocess.run(["docker", "ps", "-aq", "--filter", f"label=docklite.domain={domain}", "--filter", "label=docklite.demo=1"],
                         capture_output=True, text=True).stdout.split()
    return out[0] if out else None


for owner, domain, kind, port, running, blurb, color in SITE_PLAN:
    if container_id(domain):
        print("exists:", domain)
        continue
    body = {"domain": domain, "template_type": kind, "include_www": True, "user_id": users[owner]}
    if port:
        body["port"] = port
    status, resp = call("POST", "/api/containers", body)
    print(("created: " if status < 300 else f"FAILED ({status} {resp.get('error')}): ") + domain)
    if status >= 300:
        continue
    if kind == "static":
        folder = os.path.join(SITES, owner, domain)
        if os.path.isdir(folder):
            with open(os.path.join(folder, "index.html"), "w") as f:
                f.write(page(domain, blurb, color))
    if not running:
        cid = container_id(domain)
        if cid:
            call("POST", f"/api/containers/{cid}/stop")

for name, owner in (("shop_db", "alice"), ("blog_db", "alice"), ("analytics", "demo")):
    status, resp = call("POST", "/api/databases", {"name": name, "type": "postgres", "username": name + "_user",
                                                     "password": secrets.token_urlsafe(18)})
    print(("created db: " if status < 300 else f"db {name}: ({status}) ") + name, "" if status < 300 else resp.get("error", ""))
