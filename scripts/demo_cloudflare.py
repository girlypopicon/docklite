#!/usr/bin/env python3
"""A pretend Cloudflare API for demo mode (scripts/demo.sh): a few example.* zones, DNS records and SSL
settings kept in memory. DockLite reaches it through DOCKLITE_CLOUDFLARE_API (loopback only)."""
import json, re, sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ZONES = [{"id": f"zone-{n}", "name": n, "status": "active", "account": {"id": "acct-demo", "name": "Demo account"}}
         for n in ("example.com", "example.net", "example.org")]
RECORDS = {
    "zone-example.com": [{"id": "r1", "type": "A", "name": "example.com", "content": "203.0.113.10", "ttl": 1, "proxied": True},
                         {"id": "r2", "type": "MX", "name": "example.com", "content": "mail.example.com", "ttl": 300, "proxied": False, "priority": 10}],
    "zone-example.net": [{"id": "r3", "type": "A", "name": "portfolio.example.net", "content": "203.0.113.10", "ttl": 1, "proxied": True}],
    "zone-example.org": [],
}
SETTINGS = {z["id"]: {"ssl": "full", "always_use_https": "on"} for z in ZONES}
NEXT = [100]


class H(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def reply(self, result, code=200, ok=True):
        body = json.dumps({"success": ok, "errors": [] if ok else [{"code": 1000, "message": str(result)}],
                           "result": result if ok else None, "result_info": {"page": 1, "total_pages": 1}}).encode()
        self.send_response(code); self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body))); self.end_headers(); self.wfile.write(body)

    def body(self):
        n = int(self.headers.get("Content-Length") or 0)
        return json.loads(self.rfile.read(n) or b"{}") if n else {}

    def route(self, method):
        path = self.path.split("?")[0]
        if path == "/user/tokens/verify":
            return self.reply({"id": "demo", "status": "active"})
        if path == "/zones" and method == "GET":
            return self.reply(ZONES)
        m = re.fullmatch(r"/zones/([^/]+)/dns_records(?:/([^/]+))?", path)
        if m:
            zid, rid = m.groups()
            recs = RECORDS.setdefault(zid, [])
            if method == "GET":
                return self.reply(recs)
            if method == "POST":
                rec = self.body(); NEXT[0] += 1; rec["id"] = f"r{NEXT[0]}"; recs.append(rec); return self.reply(rec)
            for i, r in enumerate(recs):
                if r["id"] == rid:
                    if method == "PUT":
                        new = self.body(); new["id"] = rid; recs[i] = new; return self.reply(new)
                    if method == "DELETE":
                        recs.pop(i); return self.reply({"id": rid})
            return self.reply("record not found", 404, False)
        m = re.fullmatch(r"/zones/([^/]+)/settings/([^/]+)", path)
        if m:
            zid, name = m.groups()
            st = SETTINGS.setdefault(zid, {})
            if method == "PATCH":
                st[name] = self.body().get("value", st.get(name))
            return self.reply({"id": name, "value": st.get(name, "off")})
        return self.reply("not found", 404, False)

    def do_GET(self): self.route("GET")
    def do_POST(self): self.route("POST")
    def do_PUT(self): self.route("PUT")
    def do_PATCH(self): self.route("PATCH")
    def do_DELETE(self): self.route("DELETE")


if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 3199
    ThreadingHTTPServer(("127.0.0.1", port), H).serve_forever()
