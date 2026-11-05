# Extensions

Lattice discovers channel packages under `extensions/channels/*/manifest.yaml` and starts those that have a matching in-process factory registered at build time (`server/extensions` builtins).

There is no public catalog of Lattice channel packages yet. The install tooling is ready for when packages live in their own repos or release archives.

## Local directory

```text
go run ./server/cmd/install-extension -src ./path/to/my-channel
```

## Archive (local or URL)

Zip or `.tar.gz` containing a channel package (`manifest.yaml`):

```text
go run ./server/cmd/install-extension -url ./my-channel.zip
go run ./server/cmd/install-extension -url https://example.com/releases/my-channel-1.0.0.zip
```

If the archive has several packages (or a repo root wrapper), pass `-subdir`:

```text
go run ./server/cmd/install-extension -url ./repo.zip -subdir extensions/channels/dns
```

## GitHub repository

Downloads the GitHub zipball for a ref (branch or tag) and installs the channel package found inside:

```text
go run ./server/cmd/install-extension -github owner/repo
go run ./server/cmd/install-extension -github owner/repo@v1.2.3 -subdir path/to/channel
go run ./server/cmd/install-extension -github owner/repo -ref develop -subdir channels/http
```

Set `LATTICE_EXTENSIONS_DIR` or pass `-extensions-dir` to choose the install root. The tool replaces `extensions/channels/<name>`.

After install, reload channels without a full process restart:

```text
go run ./client/cmd/client reload-extensions
```

Or use **Reload extensions** on the Listeners page. **Builtin factory registration is still required** for a new channel name to start; out-of-process / WASM loading is not implemented yet.

## Manifest fields

See `extensions/channels/http/manifest.yaml` for the current shape (`name`, `kind`, `version`, `enabled`, optional `bind_addr`).

## Still open

- Out-of-process / WASM channel runtimes
- A published registry of community channel packages
