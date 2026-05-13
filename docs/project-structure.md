# Project Structure

Recommended layout for this SDK:

```text
.
├── README.md
├── docs/
│   ├── README.md
│   ├── compatibility.md
│   ├── project-structure.md
│   └── usage.md
├── samples/
│   ├── basic_streaming/
│   ├── structured_output/
│   └── resume_thread/
├── *.go
└── *_test.go
```

The Go package stays at the module root to keep imports stable:

```go
import codex "github.com/diasYuri/go-codex-sdk"
```

The `samples/` tree contains executable examples. The `docs/` tree contains the higher-level notes that do not belong in package comments.
