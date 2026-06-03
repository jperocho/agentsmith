# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-06-03

### Added
- Hub core: `~/.agentsmith` bootstrap (0700/0600 perms), atomic `skills.json`
  manifest (temp → fsync → rename), and a cross-process file lock.
- `get <repo>[@ref]` — clone a skill into the hub, pin the exact commit, record
  resolved ref, tracking mode (`pinned` for tags / `branch`), and a sha256
  content-tree checksum. Accepts `owner/name`, https, `git@` and `file://` URLs.
- `install <skill>` — install into agents by **copy** or **symlink**, project or
  `--global` scope, with `--agent`, `--mode`, `--dry-run`, and `--yes` flags and
  an interactive agent picker.
- `update <skill>` — fetch the tracked ref, show changed files, re-resolve the
  version, and re-sync stale copy installs (confirmation gated, `--yes` to skip).
  `--ref` re-points the skill at a new tag/branch.
- `status`, `list`, `uninstall`, `remove` (`--keep-installs`), and `doctor`
  (agent detection, manifest validation, checksum/tamper + drift reporting).
- Agent adapters: Claude Code (confirmed paths) plus Cursor, Codex, and Hermes
  (using the `~/.<agent>/skills` convention; paths pending verification).
- `config.json` `defaultMode` to set the default install mode.

### Security
- Executable bits stripped from the hub clone on `get`/`update` unless
  `--allow-scripts`; checksum reflects the sanitized tree.
- Pre-register content scan rejects symlinks escaping the skill directory and
  enforces per-file (50 MiB) and total (250 MiB) size caps.
- Install targets canonicalized and allowlisted to known agent roots, resolving
  parent-chain symlinks to prevent escapes.
- Atomic copy/symlink installs; agentsmith never executes skill-provided scripts.

### Tests
- Unit coverage for repo parsing, security guards (allowlist, symlink escape,
  checksum), manifest round-trip, filesystem install primitives, the agent
  registry, the git wrapper, and the cross-process lock.

### Release
- CI workflow (`go build` / `vet` / `test -race`) and a GoReleaser-driven
  release pipeline producing a `checksums.txt` and per-platform archives on `v*`
  tags, plus a `curl | sh` installer with checksum verification.
