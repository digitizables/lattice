# Persistence methods

Lattice ships three basic always-on persistence methods for **target** hosts. Do not install an implant on the command-center machine: the teamserver refuses check-ins whose hostname matches its own (`LATTICE_ALLOW_LOCAL_AGENTS=false`) and purges leftover matching sessions on start. Lab always-on uses the Compose lab-agent service instead.

Operators choose (or the installer falls back) based on privilege. Override visible names and paths with env vars when you need a quieter footprint.

- **`service`:** Windows Service, or a native OS service via `kardianos/service`. Needs elevated / admin rights. Survives reboot and logoff.
- **`logon-task`:** Windows Scheduled Task (`ONLOGON`), or a systemd user unit. Works as a normal user (or elevated). Survives reboot at logon / user session start.
- **`startup`:** `HKCU\...\Run` value, or an `@reboot` cron line. Current user only. Survives user logon.

## Install order

`agent install` / console `persist` tries methods in this order:

1. **service** — strongest; needs elevation on Windows
2. **logon-task** — logon scheduled task / user systemd unit
3. **startup** — Run key / cron (no elevation)

Success output names the method that landed (`service`, `logon-task`, or `startup`).

## Cleanup

`cleanup` / `burn` / `uninstall-all` removes service, logon-task, and startup registrations, deletes the install directory and config/`agent.id`, schedules binary deletion, then exits.

## Opsec overrides

- **`LATTICE_SERVICE_NAME`:** Service / task / Run value name (default `LatticeAgent`)
- **`LATTICE_SERVICE_DISPLAY`:** Windows service display name
- **`LATTICE_SERVICE_DESC`:** Service description
- **`LATTICE_INSTALL_DIR`:** On-disk binary location
- **`LATTICE_CONFIG_DIR`:** `agent.id` and related state
