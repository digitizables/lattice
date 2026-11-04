# Persistence methods

Lattice ships three basic always-on persistence methods for **target** hosts. Do not install an implant on the command-center machine: the teamserver refuses check-ins whose hostname matches its own (`LATTICE_ALLOW_LOCAL_AGENTS=false`) and purges leftover matching sessions on start. Lab always-on uses the Compose lab-agent service instead.

`agent install` / console `persist` picks a method from privilege and platform, falling back when a stronger option is unavailable. Env vars below rename the visible service/task identifiers and install paths when you need distinct lab labels.

- **`service`:** Windows Service, or a native OS service via `kardianos/service`. Needs elevated / admin rights. Survives reboot and logoff.
- **`logon-task`:** Windows Scheduled Task (`ONLOGON`), or a systemd user unit. Works as a normal user (or elevated). Comes back at logon / user-session start after reboot.
- **`startup`:** `HKCU\...\Run` value, or an `@reboot` cron line. Current user only. Comes back at user logon.

## Install order

1. **service** — requires elevation on Windows
2. **logon-task** — logon scheduled task / user systemd unit
3. **startup** — Run key / cron (no elevation)

Success output reports which method was used (`service`, `logon-task`, or `startup`).

## Uninstall vs cleanup

**`uninstall`** only removes the persistence registration (service, logon task, Run key / cron, and matching native service entries). The binary and `agent.id` may still be on disk.

**`cleanup`** (aliases `burn`, `uninstall-all`) is a best-effort teardown of what Lattice installed:

- Stops and removes the registrations above
- Deletes the install directory and config/`agent.id` when possible
- Schedules deletion of the running binary, then exits

That is **not** a claim of forensic erasure. Host telemetry Lattice never wrote (execution caches, filesystem journals, shadow copies, event logs, network records, and so on) is outside the scope of this command. Treat cleanup as lab reset and operational uninstall of Lattice-managed artifacts.

## Name and path overrides

- **`LATTICE_SERVICE_NAME`:** Service / task / Run value name (default `LatticeAgent`)
- **`LATTICE_SERVICE_DISPLAY`:** Windows service display name
- **`LATTICE_SERVICE_DESC`:** Service description
- **`LATTICE_INSTALL_DIR`:** On-disk binary location
- **`LATTICE_CONFIG_DIR`:** `agent.id` and related state
