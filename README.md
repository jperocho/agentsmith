# agentsmith

[![CI](https://github.com/jperocho/agentsmith/actions/workflows/ci.yml/badge.svg)](https://github.com/jperocho/agentsmith/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/jperocho/agentsmith?sort=semver)](https://github.com/jperocho/agentsmith/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/jperocho/agentsmith.svg)](https://pkg.go.dev/github.com/jperocho/agentsmith)
[![Go version](https://img.shields.io/github/go-mod/go-version/jperocho/agentsmith)](go.mod)

A local **skills hub** for AI coding agents. Clone skill repos into one central
place (`~/.agentsmith`), track the exact version of each, and install them into
your agents (Claude Code today; Cursor / Codex / Hermes planned) by **copy** or
**symlink**.

This build covers hub core, the Claude Code adapter + install, and
update/drift, plus the cheap-critical security guards (exec-bit stripping,
content scan, checksums, target allowlisting).

## Install

Single static Go binary, no runtime dependencies (needs `git` on PATH at run
time for `get`/`update`).

### curl (recommended)

Downloads the release binary for your OS/arch and verifies its sha256 checksum:

```sh
curl -fsSL https://raw.githubusercontent.com/jperocho/agentsmith/main/install.sh | sh
```

Overrides: `AGENTSMITH_VERSION=v0.1.0` to pin a tag, `AGENTSMITH_BIN_DIR=~/bin`
to choose the install dir (defaults to `/usr/local/bin`, else `~/.local/bin`).

### go install

```sh
go install github.com/jperocho/agentsmith@latest
```

### From source

```sh
go build -o agentsmith .
install -m 0755 agentsmith ~/.local/bin/agentsmith
```

Prebuilt archives + `checksums.txt` for Linux and macOS (amd64/arm64) are
attached to each [release](https://github.com/jperocho/agentsmith/releases).
Windows is not yet supported — the hub lock uses `flock(2)`; a Windows port is
planned.

## Usage

```
agentsmith get <repo>[@ref]            clone a skill into the hub (no install)
agentsmith list                        list hub skills + versions + installs
agentsmith install <skill> [flags]     install a skill into agent(s)
agentsmith update <skill>              pull latest, re-resolve, re-sync copies
agentsmith status <skill>              show install state + drift
agentsmith uninstall <skill> [flags]   remove an install from agent(s)
agentsmith remove <skill>              remove a skill from the hub entirely
agentsmith doctor                      detect agents, validate hub integrity
```

**Repo forms:** `owner/name` · `https://host/owner/name` · `git@host:owner/name.git`
· `file:///local/path`. Append `@ref` to pin a tag or branch:
`agentsmith get owner/name@v1.4.0`.

**`install` flags:**

| Flag | Meaning |
|------|---------|
| `--global` | install to user-wide agent config (default: project-local) |
| `--mode copy\|symlink` | install mode. Default: `config.json` `defaultMode`, else auto (`symlink` where supported, else `copy`) |
| `--agent <name>` | target agent(s); repeatable or comma-separated. Omit for an interactive pick list |
| `--dry-run` | print the resolved target(s) + mode and write nothing |
| `--yes` | non-interactive; assume defaults |

**`get` flag:** `--allow-scripts` — keep executable bits in the hub clone
(default strips them).

**`update` flags:** `--ref <tag\|branch>` re-points the skill at a new ref ·
`--allow-scripts` keeps exec bits · `--yes` skips the copy-resync confirmation.

**`remove` flag:** `--keep-installs` — leave installed agent files in place
(drops tracking only).

### Example

```sh
agentsmith get jperocho/some-skill@v1.2.0     # stage in the hub
agentsmith install some-skill --agent claude-code --mode symlink
agentsmith status some-skill                   # see install + drift
agentsmith update some-skill                   # pull latest, re-sync copies
```

## How it works

- **Hub root:** `~/.agentsmith` (override with `AGENTSMITH_HOME`). Layout:
  `skills/<name>/` clones, `skills.json` manifest, `cache/`, `logs/`, `bin/`.
- **Version tracking:** every skill records its repo, resolved ref, exact
  commit, fetch time, and a sha256 checksum over its content tree.
- **copy vs symlink:** `symlink` points the agent at the hub clone (always
  current, zero duplication); `copy` duplicates files (isolated, survives hub
  deletion, but goes stale until `update` re-syncs it).
- **Tracking:** a skill is `pinned` (got via `@<tag>` — `update` only moves if
  the tag itself moved, flagged as possible force-push) or follows a `branch`
  (`update` pulls new commits). `status` shows which. `update --ref` re-points.
- **Drift:** `status` and `doctor` flag copy installs whose `installedCommit`
  lags the hub, and report any hub tree whose checksum no longer matches the
  manifest (local tampering).
- **config.json** (`~/.agentsmith/config.json`): `{"defaultMode":"symlink"}`
  sets the install mode used when `--mode` is omitted.

## Security

agentsmith treats all skill content as untrusted. Implemented:

- HTTPS/SSH clone at git defaults — no insecure flags. Exact commit pinned and
  recorded after clone.
- Pre-register **content scan**: rejects symlinks escaping the skill dir;
  per-file (50 MiB) and total (250 MiB) size caps.
- **Executable bits stripped** from the hub clone on `get`/`update` unless
  `--allow-scripts` is passed (skill content is untrusted; agentsmith never runs
  it). The stored checksum reflects the sanitized tree.
- sha256 **checksum** over the content tree for integrity / tamper detection.
- Install targets are canonicalized and **allowlisted** to known agent config
  roots — agentsmith refuses to write outside them, and resolves symlinks on the
  parent chain so a redirected target can't escape.
- Atomic writes everywhere: manifest (`temp → fsync → rename`), copy installs
  (staged temp dir → rename), symlinks (temp → rename). A file lock
  (`~/.agentsmith/.lock`) guards concurrent runs.
- Hub dir is `0700`, `skills.json` is `0600`.
- agentsmith **never executes** skill-provided scripts during get/update/install.

Deferred: signed-tag verification, deep per-file content scanning, signed
release checksums.

## Agents

Claude Code paths are confirmed. Cursor / Codex / Hermes adapters are wired up
using the common `~/.<agent>/skills/<skill>/` (global) and
`./.<agent>/skills/<skill>/` (project) convention, but those paths are **not yet
verified** against each agent's real layout — confirm with
`agentsmith install <skill> --agent <name> --dry-run` before relying on them.

## Roadmap

- Verified Cursor / Codex / Hermes adapter paths.
- Windows support (a `LockFileEx`-based hub lock).
- Signed release checksums and signed-tag verification.
