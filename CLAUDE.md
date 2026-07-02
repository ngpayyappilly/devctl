# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build          # Build binary to ./bin/devctl (injects version, git SHA, build date via ldflags)
make test           # Run all tests
make tidy           # go mod tidy
make clean          # Remove ./bin/devctl
make docker         # Build Docker image tagged with current version
make release TYPE=patch|minor|major  # Bump version, run bump_version.sh, then rebuild
```

Run a single test package:
```bash
go test ./internal/awshelper/...
```

Run with a command:
```bash
make run CMD="nw --host google.com --port 443"
```

## Architecture

`devctl` is a Cobra-based CLI. The root command is assembled in [cmd/devctl/main.go](cmd/devctl/main.go), which wires together four top-level subcommand groups:

| Subcommand | Package | What it does |
|---|---|---|
| `nw` | `internal/netcheck` | TCP reachability check with configurable timeout |
| `git` | `internal/githelper` | Thin wrappers around `git` CLI (clone, checkout, commit, push) |
| `kube` | `internal/kubehelper` | Kubernetes utilities — pod listing (via client-go), context switching, deployment restarts, log tailing |
| `aws` | `internal/awshelper` | AWS SDK v2 commands for S3, EC2, CloudFormation, and IAM |

Each package exposes a single `New<Name>Cmd() *cobra.Command` constructor that registers its subcommands internally.

**Adding a new top-level command group:** create a package under `internal/`, implement `NewXCmd() *cobra.Command`, and add `rootCmd.AddCommand(x.NewXCmd())` in `main.go`.

**kubehelper** prefers the in-cluster config and falls back to `$KUBECONFIG` / `~/.kube/config`. Some subcommands (restart, set-context, logs) shell out to `kubectl` directly rather than using the Go client.

**awshelper** loads credentials via `config.LoadDefaultConfig` (respects `AWS_PROFILE`, env vars, instance metadata in order). The `ssh-ec2` command resolves a public IP from EC2 then execs `ssh` as a subprocess.

Version metadata (`version`, `gitSha`, `buildDate`) is injected at build time via ldflags; the `version` subcommand prints them.

## Release

Releases are automated via GoReleaser (`.goreleaser.yaml`) triggered by CI. Artifacts include cross-platform binaries (linux/darwin/windows × amd64/arm64), Docker images pushed to `ghcr.io/nijogeorgep/devctl`, and a Homebrew formula. The `.releaserc` drives semantic-release for changelog generation.
