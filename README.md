# Lattice

Lattice is a personal research project exploring the design of a modern command-and-control (C2) framework.

It targets a shippable, multiplayer platform: a teamserver, an operator client, a reference agent, versioned wire contracts, and a single extension system for additional agents, channels, modules, and event hooks.

## Architecture

### Protocol

Versioned schemas for agent traffic, operator control, and extension manifests. Compatibility across releases and plugins depends on this layer.

### Server

The control plane. It authenticates operators, manages listeners through channel plugins, schedules tasks, generates payloads, stores loot, and emits events for hooks.

### Client

The operator console. It speaks only to the server API. Client-side extensions can register extra commands without forking the console.

### Agent

The reference implant shipped with the core. It checks in over configured channels, runs tasked commands, and serves as the template other agent extensions should match.

### Extensions

Installable packages in four kinds:

- **Agents** alternate implants and builders
- **Channels** paired listeners, agent transports, and profile schemas
- **Modules** optional post-exploitation capabilities
- **Hooks** webhooks, log shippers, and other event consumers

Each extension declares a manifest, entrypoints, and the protocol versions it supports. The teamserver loads and versions them through one install path.

### Profiles

Channel and malleable settings as data files. Generation and listeners consume profile IDs rather than hard-coded endpoints.

## Status

Early design. Interfaces and layout may change before a stable release.

# License
MIT License. See [LICENSE](LICENSE) for the full text.