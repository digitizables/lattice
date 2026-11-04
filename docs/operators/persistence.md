# Persistence methods

Lattice ships three basic always-on persistence methods. Operators choose (or the installer falls back) based on privilege. Override visible names and paths with env vars when you need a quieter footprint.

| Codename | Technical ID | Platform mechanism | Typical privilege | Survives |
|----------|--------------|--------------------|-------------------|----------|
| **Keel** | `keel` | Windows Service, or native OS service via `kardianos/service` | Elevated / admin | Reboot, logoff |
| **Harbor** | `harbor` | Windows Scheduled Task (`ONLOGON`), or systemd user unit | User (or elevated) | Reboot (at logon / user session) |
| **Splice** | `splice` | `HKCU\...\Run` value, or `@reboot` cron | Current user | User logon |

## Install order

`agent install` / console `persist` tries methods in this order:

1. **Keel** — strongest, needs elevation on Windows  
2. **Harbor** — logon task / user systemd  
3. **Splice** — Run key / cron (no elevation)

Success output names the method that landed.

## Cleanup

`cleanup` / `burn` / `uninstall-all` tears down **Keel**, **Harbor**, and **Splice**, deletes the install directory and config/`agent.id`, schedules binary deletion, then exits. That wipe path is codenamed **Ash**.

## Opsec overrides

| Env | Affects |
|-----|---------|
| `LATTICE_SERVICE_NAME` | Service / task / Run value name (default `LatticeAgent`) |
| `LATTICE_SERVICE_DISPLAY` | Windows service display name |
| `LATTICE_SERVICE_DESC` | Service description |
| `LATTICE_INSTALL_DIR` | On-disk binary location |
| `LATTICE_CONFIG_DIR` | `agent.id` and related state |
