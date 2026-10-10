# Contributing

Issues and pull requests are welcome. For orientation read [CLAUDE.md](CLAUDE.md) (architecture, API, where
things live) and [docs/TESTING.md](docs/TESTING.md).

```bash
make test          # Go and web tests
make build-agent   # agent
make build-gui     # dashboard
```

Keep changes small and focused, add a test where there is logic, and update
`go-app/cmd/docklite/docs/CLI.md` when commands change.

By contributing you agree your work is released under the project's AGPL-3.0-or-later license.

## Demo mode

`scripts/demo.sh up` starts a throwaway DockLite (agent on :3100, dashboard on :3102) in `~/docklite-demo`, seeded
with fake users, `example.*` sites and databases. `scripts/demo.sh login` prints the demo login, `status` shows
whether it is running, and `down --wipe` removes it. Use it for screenshots and testing without touching a real
server: root actions are simulated in memory and pages that reveal the host (network, server logs/services) are
refused. Demo containers carry the label `docklite.demo=1`.

## Versioning

Add a line under **Unreleased** in `CHANGELOG.md` for every user-visible change as you make it. Servers update from GitHub
*releases* (the dashboard's Update button installs the newest `vX.Y.Z` tag), so a change isn't deployable until it's released.

```bash
make version                                 # where are we? flags anything out of step
scripts/release.sh prepare minor             # on your branch: bump VERSION, package.json, CHANGELOG; commit (patch|minor|major|X.Y.Z)
# ...open a PR and merge it, then on main:
scripts/release.sh tag --push                # tag vX.Y.Z and push the tag
scripts/release.sh publish                   # create the GitHub release with the changelog notes
```

Rule of thumb: `patch` for fixes, `minor` for new features, and `1.5.0` is the "production ready" goal.
