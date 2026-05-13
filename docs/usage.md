# Usage

## Client

```go
client, err := codex.New(&codex.CodexOptions{
	BaseURL: "https://example.test",
})
if err != nil {
	log.Fatal(err)
}
```

## Thread

```go
thread := client.StartThread(&codex.ThreadOptions{
	Model: "gpt-test-1",
})

turn, err := thread.Run(context.Background(), codex.TextInput("Diagnose the test failure"), nil)
```

## Streaming

```go
events, errs := thread.RunStreamed(ctx, codex.TextInput("Implement the fix"), nil)
for event := range events {
	fmt.Println(event.Type)
}
if err := <-errs; err != nil {
	log.Fatal(err)
}
```

## Structured Input

```go
turn, err := thread.Run(ctx, codex.StructuredInput{
	codex.TextPart{Text: "Describe these screenshots"},
	codex.LocalImagePart{Path: "./ui.png"},
}, nil)
```

## Output Schema

Pass a JSON object in `TurnOptions.OutputSchema` to forward `--output-schema` to the CLI.

## Binary Resolution

If `CodexOptions.CodexPathOverride` is empty, the SDK resolves `codex` from `PATH`.
