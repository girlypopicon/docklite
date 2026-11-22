# Installing DockLite

## Requirements

- Ubuntu or Debian (other apt-based distros usually work), 1 GB RAM or more, root or `sudo`.
- Ports 80 and 443 free if you want sites and the dashboard on a real domain.
- Nothing else: the installer adds Docker, Node.js, Go (for building), nginx, certbot and PM2 when missing.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/girlypopicon/docklite/main/get.sh | bash
```

That downloads DockLite to `/usr/local/src/docklite` and starts the installer, which asks its questions at your
keyboard. To choose a version instead of `main`, add `--ref v1.1.0` after `bash -s --`. If you would rather read the
code first, clone the repository and run `sudo bash install.sh` yourself.

The installer, in order: installs missing packages, creates the `docklite` system user and `/opt/docklite`,
builds the agent, the `docklite` command line and the dashboard, copies everything to `/opt/docklite`, installs
the root helper and its sudoers rule, then starts the setup wizard.

The wizard asks for:

1. **Ports.** The agent defaults to 3000 and the dashboard to 3002.
2. **Mode.** *Full Stack* (dashboard + agent, recommended) or *Headless* (agent and terminal/API only).
3. **nginx.** Whether to serve DockLite on port 80 instead of the agent port.
4. **A dashboard domain** (optional). If you give one, DockLite requests an HTTPS certificate for it.
5. **Firewall.** Whether to open the ports DockLite needs.

## First login

The username is `superadmin`. The password is generated on first start:

```bash
sudo cat /opt/docklite/data/initial-admin-password
```

Change it from the dashboard after logging in.

## Upgrading

The easiest way: **Settings → System → DockLite Updates**. It shows the newest release and what's new, and updates with one
click: it backs up your data first, and goes back to the previous version by itself if the new one doesn't start.

From a terminal instead:

Run the same install command again. It fetches the new code and upgrades in place, keeping your database, logs
and configuration:

```bash
curl -fsSL https://raw.githubusercontent.com/girlypopicon/docklite/main/get.sh | bash
```

## After installing

```bash
docklite doctor        # health check
docklite status        # services
docklite repair        # health plus a check of how site folders are laid out
```

## Uninstalling

```bash
sudo bash uninstall.sh
```

It asks before removing services, configuration, and (separately) your data.

## Installing on a server that already hosts sites

First look, without changing anything:

```bash
curl -fsSL https://raw.githubusercontent.com/girlypopicon/docklite/main/get.sh | bash -s -- --dry-run
bash /usr/local/src/docklite/inventory.sh   # a fuller read-only report of nginx sites, ports and containers
```

On a server with existing sites the installer:

- keeps every existing nginx config enabled (including an older DockLite's `docklite-sites`) and never adds a
  second `default_server`; if one exists, DockLite is reached through its panel domain or its own port;
- reloads nginx once, only after `nginx -t` passes, and undoes its own change if the test fails;
- backs up an older DockLite found in `/opt/docklite` to `/var/backups/docklite/` before replacing its files;
- does not move or re-own anything in `/var/www/sites`, and does not stop, restart or recreate containers.

Afterwards, `docklite sites layout` shows whether sites follow the `/var/www/sites/<user>/<domain>/` layout.

## Several servers in one dashboard

Install DockLite on each server first. The agent alone is enough (`--terminal-only` skips the dashboard); the web dashboard and terminal app on the
other servers are optional. Pick one server as the hub (the one whose dashboard you open), then:

1. On the other server: `docklite share`. It prints a join code with that server's address and a token made for the hub.
2. On the hub: **Add server** in the dashboard (paste the code), or `docklite servers add <code>`.

The other server needs to be reachable from the hub. Give it a domain with HTTPS first (`docklite panel set smoll.docklite.net --enable`, then
`docklite ssl issue smoll.docklite.net`) so the token isn't sent in plain text; on a private network `http://<ip>:3000` is fine. To cut a hub off, revoke the
token named `hub: ...` on the other server.
