# Installing DockLite

## Requirements

- Ubuntu or Debian (other apt-based distros usually work), 1 GB RAM or more, root or `sudo`.
- Ports 80 and 443 free if you want sites and the dashboard on a real domain.
- Nothing else: the installer adds Docker, Node.js, Go (for building), nginx, certbot and PM2 when missing.

## Install

```bash
git clone https://github.com/girlypopicon/docklite.git
cd docklite
sudo bash install.sh
```

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

Pull the new code and run the installer again. It keeps your database, logs and configuration:

```bash
cd docklite && git pull
sudo bash install.sh
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

Run `bash inventory.sh` first. It only reads, and prints what is installed, which nginx sites go to which ports
and which containers are present. DockLite's installer does not touch existing nginx site files or re-own
existing site folders.
