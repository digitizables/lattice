# Extensions

Installable Lattice packages. The teamserver discovers channel manifests under `channels/*/manifest.yaml` and starts any that are enabled when a matching in-process factory is registered.

## Channel manifest

```yaml
name: http
kind: channel
version: 0.1.0
enabled: true
# bind_addr: ":8443"   # optional; defaults to LATTICE_AGENT_ADDR
protocol:
  min: lattice.agent.v1
description: ...
entrypoint: channel.go
```

Set `enabled: false` to skip a shipped channel without deleting it.

## Builtins

Factories for extensions that ship with Lattice are registered in `server/extensions/builtins.go`. A manifest alone is not enough until a factory (or a future out-of-process loader) exists for that name.

## Env

- `LATTICE_EXTENSIONS_DIR` — root directory containing `channels/` (default `extensions`)
- `LATTICE_CERT_DIR` — where lab agent-channel TLS certs are written (default `data/certs`)
- `LATTICE_AGENT_CLEARTEXT=true` — disable TLS for the builtin HTTP channel (lab only)
