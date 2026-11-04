# Bring-your-own elevation modules

Lattice does **not** ship privilege-escalation or UAC-bypass payloads. Operators publish their own modules to the teamserver; agents pull and execute them on demand.

Use only against systems you own or are explicitly authorized to test.

## Module contract

A module is an executable (or script launcher) for a given `os`/`arch`:

1. Published into `data/modules/` with a name, version, SHA-256, and optional default args  
2. Tasked with `elevate <name> [extra args…]`  
3. Agent downloads, verifies hash, runs with the agent’s current working directory  
4. Stdout/stderr and exit code return to the operator  
5. The temp copy is deleted after execution  

Exit code `0` means success from the operator’s point of view. Non-zero is failure. Lattice does not interpret tokens or inject into other processes for you—that stays inside your module.

## Publish

```text
go run ./server/cmd/publish-module -name my-uac -version 1.0.0 -os windows -arch amd64 -file .\path\to\your.exe
```

Optional default args (JSON array string):

```text
go run ./server/cmd/publish-module -name my-uac -version 1.0.0 -os windows -arch amd64 -file .\tool.exe -args "[\"--silent\"]"
```

## Task

```text
elevate my-uac
elevate my-uac --flag value
```

List what the catalog has (operator API): `ListElevateModules`.

## Lab example

`examples/elevate-noop` is a tiny program that only prints the host identity and exits 0. It proves the pipeline without performing elevation.
