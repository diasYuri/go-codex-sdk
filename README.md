# Go Codex SDK

Go port of the TypeScript Codex SDK. It wraps the `codex` CLI, sends prompts over stdin, and reads JSONL events from stdout.

## Layout

- `docs/`: architecture, API notes, and compatibility guidance
- `samples/`: runnable examples for common SDK flows
- `/`: public Go package

## Start

See [docs/usage.md](docs/usage.md) for the API surface and [samples/](samples) for runnable examples.

## CLI Resolution

Use `CodexOptions.CodexPathOverride` to point to a specific `codex` binary. If it is empty, the SDK resolves `codex` from `PATH`.
