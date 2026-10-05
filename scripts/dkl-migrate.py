#!/usr/bin/env python3
"""
dkl-migrate  —  Retroactively generate .dkl manifests and/or .dklpkg archives
for every DockLite-managed site container on this machine.

Works on old installations that pre-date the .dkl/onboard feature, as long as
the containers are still present (running or stopped) in Docker.

Usage
-----
  # Write a .dkl manifest into each site directory (safe, non-destructive):
  python3 dkl-migrate.py

  # Also create a .dklpkg archive (full snapshot) per site:
  python3 dkl-migrate.py --pkg

  # Write .dklpkg files to a specific output directory:
  python3 dkl-migrate.py --pkg --out /mnt/backup

  # Preview what would happen without writing anything:
  python3 dkl-migrate.py [--pkg] --dry-run

Requirements: Python 3.6+, docker CLI accessible, no extra packages needed.
If Docker requires root, run with: sudo python3 dkl-migrate.py
"""

import argparse
import datetime
import io
import json
import os
import subprocess
import sys
import tarfile
from pathlib import Path

MANIFEST_FILENAME = ".dkl"
MANIFEST_VERSION  = "1"

# Maps known DockLite site image prefixes to template types
SITE_IMAGE_MAP = {
    "nginx:alpine":                   "static",
    "nginx":                          "static",
    "webdevops/php-nginx:8.2-alpine": "php",
    "webdevops/php-nginx":            "php",
    "node:20-alpine":                 "node",
    "node:18-alpine":                 "node",
    "node:lts":                       "node",
    "node:alpine":                    "node",
    "node":                           "node",
}

# Container-internal file paths per template type
SITE_CONTAINER_PATHS = {
    "static": ["/usr/share/nginx/html", "/var/www/html"],
    "php":    ["/var/www/html", "/app"],
    "node":   ["/app", "/usr/src/app", "/home/node/app"],
}

# Env var key substrings to strip from manifests (no secrets in manifests)
SECRET_KEY_FRAGMENTS = ("password", "secret", "key", "token", "pw", "pass",
                        "database_url", "db_url", "dsn")


# ── Docker helpers ────────────────────────────────────────────────────────────

def docker_run(*args) -> subprocess.CompletedProcess:
    return subprocess.run(["docker", *args], capture_output=True, text=True)


def list_all_container_ids() -> list:
    r = docker_run("ps", "-aq")
    if r.returncode != 0:
        return []
    return [line.strip() for line in r.stdout.strip().splitlines() if line.strip()]


def inspect_container(cid: str) -> dict:
    r = docker_run("inspect", cid)
    if r.returncode != 0:
        return {}
    try:
        data = json.loads(r.stdout)
        return data[0] if data else {}
    except (json.JSONDecodeError, IndexError):
        return {}


# ── Classification ────────────────────────────────────────────────────────────

def get_labels(insp: dict) -> dict:
    return (insp.get("Config") or {}).get("Labels") or {}


def get_image(insp: dict) -> str:
    return (insp.get("Config") or {}).get("Image", "")


def is_db_container(insp: dict) -> bool:
    labels = get_labels(insp)
    if labels.get("docklite.type") in ("postgres", "mysql", "mongo", "mongodb", "redis"):
        return True
    if labels.get("docklite.database"):
        return True
    image = get_image(insp)
    return any(db in image.lower() for db in ("postgres", "mysql", "mongo", "redis", "mariadb"))


def is_site_container(insp: dict) -> bool:
    labels = get_labels(insp)
    # Explicit DockLite site label (modern installs)
    if labels.get("docklite.managed") == "true":
        return not is_db_container(insp)
    # Heuristic for old installs: known site image + bind mount to a web path
    image = get_image(insp)
    is_known_site_image = any(image.startswith(prefix) for prefix in SITE_IMAGE_MAP)
    if is_known_site_image:
        for m in (insp.get("Mounts") or []):
            if m.get("Type") != "bind":
                continue
            src = m.get("Source", "")
            dst = m.get("Destination", "")
            skip = ("/var/run/docker.sock", "/etc/hosts", "/etc/resolv.conf",
                    "/etc/localtime", "/etc/timezone")
            if src in skip or dst in skip:
                continue
            if any(p in src for p in ("/var/www", "/sites", "/html", "/app")):
                return True
            all_dst = [p for paths in SITE_CONTAINER_PATHS.values() for p in paths]
            if dst in all_dst:
                return True
    return False


# ── Metadata extraction ───────────────────────────────────────────────────────

def infer_template_type(insp: dict) -> str:
    labels = get_labels(insp)
    if labels.get("docklite.type") in ("static", "php", "node"):
        return labels["docklite.type"]
    image = get_image(insp)
    for prefix, ttype in SITE_IMAGE_MAP.items():
        if image.startswith(prefix):
            return ttype
    if "php" in image.lower():
        return "php"
    if "node" in image.lower():
        return "node"
    return "static"


def find_site_path(insp: dict, template_type: str) -> str:
    """Return the host bind-mount path for the site files, or empty string."""
    mounts = insp.get("Mounts") or []
    preferred_dst = SITE_CONTAINER_PATHS.get(template_type, ["/var/www/html", "/app"])

    # First pass: exact match on preferred container destination
    for dst in preferred_dst:
        for m in mounts:
            if m.get("Type") == "bind" and m.get("Destination") == dst:
                return m.get("Source", "")

    # Second pass: any bind mount that looks like site files
    skip_srcs = {"/var/run/docker.sock", "/etc/hosts", "/etc/resolv.conf",
                 "/etc/localtime", "/etc/timezone"}
    for m in mounts:
        if m.get("Type") != "bind":
            continue
        src = m.get("Source", "")
        if src in skip_srcs or src.startswith("/sys") or src.startswith("/proc"):
            continue
        if any(p in src for p in ("/var/www", "/sites", "/html")):
            return src

    return ""


def extract_domain(insp: dict) -> str:
    labels = get_labels(insp)
    if labels.get("docklite.domain"):
        return labels["docklite.domain"]
    name = (insp.get("Name") or "").lstrip("/")
    return name


def extract_port(insp: dict, template_type: str) -> int:
    labels = get_labels(insp)
    if labels.get("docklite.internal_port"):
        try:
            return int(labels["docklite.internal_port"])
        except ValueError:
            pass
    # Scan PortBindings for the first non-system port
    port_bindings = (insp.get("HostConfig") or {}).get("PortBindings") or {}
    for port_proto in sorted(port_bindings.keys()):
        num = int(port_proto.split("/")[0])
        if num not in (22, 443, 80):
            return num
        if num == 80 and template_type in ("static", "php"):
            return 80
    return 3000 if template_type == "node" else 80


def extract_username(insp: dict) -> str:
    labels = get_labels(insp)
    return labels.get("docklite.username", "")


def extract_include_www(insp: dict) -> bool:
    return get_labels(insp).get("docklite.include_www", "false").lower() == "true"


def extract_safe_env(insp: dict) -> list:
    """Return env vars stripped of anything that looks like a secret."""
    env = (insp.get("Config") or {}).get("Env") or []
    system_keys = {"PATH", "HOME", "HOSTNAME", "TERM", "LANG", "LC_ALL"}
    result = []
    for entry in env:
        if "=" not in entry:
            continue
        key, _ = entry.split("=", 1)
        if key in system_keys:
            continue
        if any(frag in key.lower() for frag in SECRET_KEY_FRAGMENTS):
            continue
        result.append(entry)
    return result


def build_manifest(insp: dict) -> dict:
    template_type = infer_template_type(insp)
    port = extract_port(insp, template_type)
    internal_port = 80 if template_type in ("static", "php") else port
    created_raw = insp.get("Created", "")
    # Normalise to RFC3339 (Go time.Time expects this)
    if created_raw:
        try:
            # Docker may return sub-second precision; truncate to seconds
            dt = datetime.datetime.fromisoformat(created_raw.replace("Z", "+00:00"))
            created_at = dt.strftime("%Y-%m-%dT%H:%M:%SZ")
        except ValueError:
            created_at = created_raw
    else:
        created_at = datetime.datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ")

    return {
        "version":      MANIFEST_VERSION,
        "domain":       extract_domain(insp),
        "templateType": template_type,
        "image":        get_image(insp),
        "internalPort": internal_port,
        "port":         port if template_type == "node" else 0,
        "includeWww":   extract_include_www(insp),
        "username":     extract_username(insp),
        "env":          extract_safe_env(insp),
        "createdAt":    created_at,
    }


# ── File writers ──────────────────────────────────────────────────────────────

def write_manifest(site_path: str, manifest: dict, dry_run: bool) -> None:
    dest = os.path.join(site_path, MANIFEST_FILENAME)
    if dry_run:
        print(f"    [dry-run] would write {dest}")
        return
    with open(dest, "w") as f:
        json.dump(manifest, f, indent=2)
        f.write("\n")
    print(f"    wrote {dest}")


def write_dklpkg(site_path: str, manifest: dict, out_dir: str, dry_run: bool) -> None:
    safe_domain = manifest.get("domain", "unknown").replace("/", "_").replace(":", "_")
    filename = f"{safe_domain}.dklpkg"
    dest = os.path.join(out_dir or site_path, filename)

    if dry_run:
        print(f"    [dry-run] would create {dest}")
        return

    manifest_bytes = (json.dumps(manifest, indent=2) + "\n").encode()

    with open(dest, "wb") as fh:
        with tarfile.open(fileobj=fh, mode="w:gz") as tw:
            # 1. manifest.dkl entry
            hdr = tarfile.TarInfo(name="manifest.dkl")
            hdr.size = len(manifest_bytes)
            hdr.mode = 0o644
            tw.addfile(hdr, io.BytesIO(manifest_bytes))

            # 2. Site files under files/
            skipped = 0
            for root, dirs, files in os.walk(site_path):
                dirs[:] = [d for d in sorted(dirs) if not d.startswith(".")]
                for fname in sorted(files):
                    if fname == MANIFEST_FILENAME:
                        continue
                    fpath = os.path.join(root, fname)
                    rel = os.path.relpath(fpath, site_path)
                    try:
                        tw.add(fpath, arcname=f"files/{rel}", recursive=False)
                    except (OSError, PermissionError):
                        skipped += 1

    size_mb = os.path.getsize(dest) / (1024 * 1024)
    msg = f"    created {dest} ({size_mb:.1f} MB)"
    if skipped:
        msg += f" [{skipped} files skipped: permission denied]"
    print(msg)


# ── DB summary ────────────────────────────────────────────────────────────────

def summarize_db(insp: dict) -> dict:
    labels = get_labels(insp)
    name = (insp.get("Name") or "").lstrip("/")
    return {
        "name":      labels.get("docklite.database", name),
        "type":      labels.get("docklite.type", "unknown"),
        "image":     get_image(insp),
        "container": name,
    }


# ── Main ──────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(
        description="Generate .dkl manifests and .dklpkg archives for existing DockLite containers",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument("--pkg",     action="store_true", help="Also generate a .dklpkg archive per site")
    parser.add_argument("--out",     metavar="DIR",       help="Output directory for .dklpkg files (default: site directory)")
    parser.add_argument("--dry-run", action="store_true", help="Preview only — don't write any files")
    args = parser.parse_args()

    if args.out and not args.pkg:
        print("[warn] --out has no effect without --pkg", file=sys.stderr)

    if args.out and not args.dry_run:
        os.makedirs(args.out, exist_ok=True)

    # Verify Docker is reachable
    check = docker_run("info")
    if check.returncode != 0:
        print("[error] Cannot connect to Docker daemon.", file=sys.stderr)
        print("        Is Docker running? You may need to run with sudo.", file=sys.stderr)
        sys.exit(1)

    ids = list_all_container_ids()
    if not ids:
        print("No containers found.")
        return

    print(f"Inspecting {len(ids)} container(s)…\n")

    sites_ok      = 0
    sites_skipped = 0
    dbs_found     = []
    already_had   = 0

    for cid in ids:
        insp = inspect_container(cid)
        if not insp:
            continue

        if is_db_container(insp):
            dbs_found.append(summarize_db(insp))
            continue

        if not is_site_container(insp):
            continue

        domain        = extract_domain(insp)
        template_type = infer_template_type(insp)
        site_path     = find_site_path(insp, template_type)

        print(f"[{domain}]  template={template_type}  image={get_image(insp)}")

        if not site_path:
            print(f"  SKIP — could not find bind-mounted site path")
            print(f"         Run: docker inspect {cid[:12]}  and look for Mounts")
            sites_skipped += 1
            continue

        if not os.path.isdir(site_path):
            print(f"  SKIP — path does not exist on disk: {site_path}")
            sites_skipped += 1
            continue

        print(f"  path  : {site_path}")

        # Check if manifest already exists
        existing = os.path.join(site_path, MANIFEST_FILENAME)
        if os.path.exists(existing) and not args.dry_run:
            print(f"  .dkl already present — skipping manifest write")
            with open(existing) as f:
                manifest = json.load(f)
            already_had += 1
        else:
            manifest = build_manifest(insp)
            write_manifest(site_path, manifest, args.dry_run)

        if args.pkg:
            write_dklpkg(site_path, manifest, args.out, args.dry_run)

        sites_ok += 1
        print()

    # ── Summary ───────────────────────────────────────────────────────────────
    print("─" * 56)
    print(f"Sites processed : {sites_ok}"
          + (f" ({already_had} already had a .dkl)" if already_had else ""))
    if sites_skipped:
        print(f"Sites skipped   : {sites_skipped} (no bind-mount path found)")

    if dbs_found:
        print(f"\nDatabases found ({len(dbs_found)}) — not exported by this tool.")
        print("Use the Backups tab in DockLite to back them up.")
        for db in dbs_found:
            print(f"  {db['name']:<30} type={db['type']}  image={db['image']}")

    if not args.pkg:
        print("\nTip: run with --pkg to also generate portable .dklpkg archives.")
    elif args.out:
        print(f"\n.dklpkg archives written to: {args.out}")


if __name__ == "__main__":
    main()
