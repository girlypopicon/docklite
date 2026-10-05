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

## Versioning

Add a line under **Unreleased** in `CHANGELOG.md` for every user-visible change as you make it.

```bash
make version                       # where are we? flags anything out of step
scripts/release.sh minor           # or patch / major / X.Y.Z; add --dry-run to preview
```

The script moves Unreleased under the new version, updates `VERSION` and `webapp/package.json`, commits
`Release vX.Y.Z` and tags it locally. It never pushes; it prints the push commands. Release from `main` after
merging. Rule of thumb: `patch` for fixes, `minor` for new features, and `1.5.0` is the "production ready" goal.
