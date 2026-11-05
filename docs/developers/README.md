# Developer documentation

Lattice is an extensible C2 monorepo: a Go teamserver, reference agent, operator CLI, Svelte web console, Protobuf contracts, channel extensions, and Docker Compose lab packaging.

This tree is the developer source of truth. Operator-facing runbooks live under [`../operators/`](../operators/).

## Contents

- [Getting started](getting-started.md) — stack, lab run, build order, checklist
- [Architecture](architecture.md) — deployable units and request paths
- [Monorepo layout](monorepo.md) — packages and responsibilities
- [Protocol](protocol.md) — Protobuf, Connect, agent HTTP channel
- [Teamserver](teamserver.md) — config, store, core, API, listen, updates, modules
- [Agent](agent.md) — identity, transport, shell, persist, cleanup, update, elevate
- [Custom agents](custom-agents.md) — third-party implant contract (Protobuf + token)
- [Operator CLI](operator-cli.md) — `client` commands
- [Operator web UI](operator-ui.md) — Svelte console
- [Extensions](extensions.md) — manifests, install, and reload
- [Data model](data-model.md) — PostgreSQL schema
- [Compose and deploy](compose-and-deploy.md) — Docker images and lab agent
- [Development workflow](development-workflow.md) — proto, tests, publishing artifacts
- [Product roadmap](roadmap.md) — Done / Next / Later tracker

## Diagrams

Architecture and flow figures are PNG exports under [`images/`](images/). They are generated from FigJam board [Lattice developer docs diagrams](https://www.figma.com/board/pgKRBDPTfSTomv34YhCNy3) (file key `pgKRBDPTfSTomv34YhCNy3`).
