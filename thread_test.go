package codex

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestThreadRunReturnsCompletedTurnAndStoresThreadID(t *testing.T) {
	client, _ := newFakeClient(t, fakeClientOptions{
		stdout: strings.Join([]string{
			`{"type":"thread.started","thread_id":"thread_123"}`,
			`{"type":"turn.started"}`,
			`{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"Hi!"}}`,
			`{"type":"turn.completed","usage":{"input_tokens":42,"cached_input_tokens":12,"output_tokens":5,"reasoning_output_tokens":0}}`,
		}, "\n"),
	})

	thread := client.StartThread(nil)
	turn, err := thread.Run(context.Background(), TextInput("Hello"), nil)
	if err != nil {
		t.Fatal(err)
	}

	if turn.FinalResponse != "Hi!" {
		t.Fatalf("final response mismatch: %q", turn.FinalResponse)
	}
	if len(turn.Items) != 1 {
		t.Fatalf("items mismatch: %#v", turn.Items)
	}
	if turn.Usage == nil || turn.Usage.InputTokens != 42 || turn.Usage.CachedInputTokens != 12 {
		t.Fatalf("usage mismatch: %#v", turn.Usage)
	}
	if id, ok := thread.ID(); !ok || id != "thread_123" {
		t.Fatalf("thread id mismatch: %q %t", id, ok)
	}
}

func TestThreadRunStreamedReturnsEvents(t *testing.T) {
	client, _ := newFakeClient(t, fakeClientOptions{
		stdout: strings.Join([]string{
			`{"type":"thread.started","thread_id":"thread_123"}`,
			`{"type":"turn.started"}`,
			`{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"Hi!"}}`,
			`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":2,"reasoning_output_tokens":0}}`,
		}, "\n"),
	})

	thread := client.StartThread(nil)
	events, errs := thread.RunStreamed(context.Background(), TextInput("Hello"), nil)

	var got []ThreadEvent
	for event := range events {
		got = append(got, event)
	}
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("events mismatch: %#v", got)
	}
	if got[0].Type != "thread.started" || got[0].ThreadID != "thread_123" {
		t.Fatalf("thread started event mismatch: %#v", got[0])
	}
	if item, ok := got[2].Item.(*AgentMessageItem); !ok || item.Text != "Hi!" {
		t.Fatalf("agent item mismatch: %#v", got[2].Item)
	}
}

func TestThreadResumeAndImagesAreForwarded(t *testing.T) {
	argsPath := filepath.Join(t.TempDir(), "args.txt")
	client, _ := newFakeClient(t, fakeClientOptions{
		argsPath: argsPath,
		stdout: strings.Join([]string{
			`{"type":"turn.started"}`,
			`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":2,"reasoning_output_tokens":0}}`,
		}, "\n"),
	})

	thread := client.ResumeThread("thread-id", nil)
	_, err := thread.Run(context.Background(), StructuredInput{
		TextPart{Text: "describe"},
		LocalImagePart{Path: "first.png"},
		LocalImagePart{Path: "second.jpg"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	args := strings.Fields(readFile(t, argsPath))
	resumeIndex := indexOf(args, "resume")
	imageIndex := indexOf(args, "--image")
	if resumeIndex == -1 || imageIndex == -1 || resumeIndex > imageIndex {
		t.Fatalf("resume must appear before image args: %#v", args)
	}
	if images := valuesAfter(args, "--image"); !reflect.DeepEqual(images, []string{"first.png", "second.jpg"}) {
		t.Fatalf("images mismatch: %#v", images)
	}
}

func TestStructuredInputCombinesTextSegments(t *testing.T) {
	input := StructuredInput{
		TextPart{Text: "Describe file changes"},
		TextPart{Text: "Focus on impacted tests"},
		LocalImagePart{Path: "ui.png"},
	}
	prompt, images := input.normalizeCodexInput()
	if prompt != "Describe file changes\n\nFocus on impacted tests" {
		t.Fatalf("prompt mismatch: %q", prompt)
	}
	if !reflect.DeepEqual(images, []string{"ui.png"}) {
		t.Fatalf("images mismatch: %#v", images)
	}
}

func TestOutputSchemaIsForwardedAndCleanedUp(t *testing.T) {
	argsPath := filepath.Join(t.TempDir(), "args.txt")
	client, _ := newFakeClient(t, fakeClientOptions{
		argsPath: argsPath,
		stdout: strings.Join([]string{
			`{"type":"turn.started"}`,
			`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":2,"reasoning_output_tokens":0}}`,
		}, "\n"),
	})

	thread := client.StartThread(nil)
	_, err := thread.Run(context.Background(), TextInput("structured"), &TurnOptions{
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"answer": map[string]any{"type": "string"},
			},
			"required":             []string{"answer"},
			"additionalProperties": false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	args := strings.Fields(readFile(t, argsPath))
	schemaIndex := indexOf(args, "--output-schema")
	if schemaIndex == -1 || schemaIndex+1 >= len(args) {
		t.Fatalf("schema arg missing: %#v", args)
	}
	if _, err := os.Stat(args[schemaIndex+1]); !os.IsNotExist(err) {
		t.Fatalf("schema file should be cleaned up, stat err: %v", err)
	}
}

func TestThreadRunReturnsTurnFailure(t *testing.T) {
	client, _ := newFakeClient(t, fakeClientOptions{
		stdout: strings.Join([]string{
			`{"type":"turn.started"}`,
			`{"type":"turn.failed","error":{"message":"rate limit exceeded"}}`,
		}, "\n"),
	})

	thread := client.StartThread(nil)
	_, err := thread.Run(context.Background(), TextInput("fail"), nil)
	if err == nil || !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Fatalf("expected turn failure, got %v", err)
	}
}

type fakeClientOptions struct {
	argsPath string
	stdout   string
}

func newFakeClient(t *testing.T, options fakeClientOptions) (*Codex, string) {
	t.Helper()
	dir := t.TempDir()
	fake := writeFakeCodex(t, dir)
	env := map[string]string{
		"FAKE_CODEX_STDOUT": options.stdout,
	}
	if options.argsPath != "" {
		env["FAKE_CODEX_ARGS_FILE"] = options.argsPath
	}

	client, err := New(&CodexOptions{
		CodexPathOverride: fake,
		Env:               env,
	})
	if err != nil {
		t.Fatal(err)
	}
	return client, fake
}
