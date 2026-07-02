# Changelog

All notable changes to this project will be documented in this file.

This file is generated automatically by [semantic-release](https://github.com/semantic-release/semantic-release).

## [Unreleased]

### Features

#### Authentication Framework (`devctl auth`)
- New `devctl auth` command group with `login`, `logout`, `status`, and `token` subcommands
- Pluggable provider architecture — organizations select their IdP via `auth.provider` in config; no code changes needed to switch
- OS keychain storage for sessions (`go-keyring`: macOS Keychain, Linux Secret Service, Windows DPAPI) with automatic file fallback at `~/.devctl/sessions/<provider>.json`
- **API key / service account provider** (`--provider apikey`): non-interactive authentication for CI/CD pipelines; token resolved via `--token` flag → `DEVCTL_TOKEN` env var → `auth.token` config key; sessions never expire
- Provider registry for runtime lookup and extensibility

#### Configuration Management (`devctl config`)
- New `devctl config init` command generates `~/.devctl/config.yaml` from a commented template (idempotent)
- Global `--config` flag to specify an alternate config file path
- Config file search order: `--config` flag → `.devctl.yaml` in CWD → `~/.devctl/config.yaml`
- Viper integration with environment variable support and precedence: CLI flag > env var > config file > default

### Enhancements

- `devctl aws ssh-ec2`: `--region` and `--user` flags now fall back to `defaults.aws_region` and `defaults.ssh_username` from config/env
- `devctl kube get-pods`: `--namespace` flag now falls back to `defaults.kube_namespace` from config/env
- `AWS_REGION` and `AWS_DEFAULT_REGION` environment variables honoured as aliases for `defaults.aws_region`
- `DEVCTL_TOKEN` environment variable bound to `auth.token` config key
