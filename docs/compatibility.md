# Compatibility

This SDK keeps the TypeScript behavior as the reference, but the Go API is idiomatic:

- `context.Context` is used for cancellation instead of `AbortSignal`
- streaming returns channels instead of an async generator
- `TextInput` and `StructuredInput` replace the TypeScript union input type

Compatibility points that are preserved:

- `codex exec --experimental-json`
- JSONL event parsing
- `--config` flattening and TOML serialization
- `--output-schema` temp file lifecycle
- `--image` forwarding and resume behavior
- originator and API key environment injection
