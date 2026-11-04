# Extensions

Lattice discovers channel packages under `extensions/channels/*/manifest.yaml` and starts those that have a matching in-process factory registered at build time (`server/extensions` builtins).

## Local install

Copy a channel package (directory with `manifest.yaml`) into the extensions tree:

```text
go run ./server/cmd/install-extension -src ./path/to/my-channel
```

Or set `LATTICE_EXTENSIONS_DIR` and pass `-extensions-dir`. The tool replaces `extensions/channels/<name>` with the source package.

Restart the teamserver after install. **Builtin factory registration is still required** for the channel to start; remote/WASM/plugin loading is not implemented yet.

## Manifest fields

See `extensions/channels/http/manifest.yaml` for the current shape (`name`, `kind`, `version`, `enabled`, optional `bind_addr`).

## Not yet implemented

- Install from a GitHub repository or release archive
- Hot-reload without process restart
- Out-of-process / WASM channel runtimes
