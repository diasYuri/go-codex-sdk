package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

type Turn struct {
	Items         []ThreadItem
	FinalResponse string
	Usage         *Usage
}

type Thread struct {
	exec          *codexExec
	options       CodexOptions
	threadOptions ThreadOptions

	mu sync.RWMutex
	id string
}

func newThread(exec *codexExec, options CodexOptions, threadOptions ThreadOptions, id string) *Thread {
	return &Thread{
		exec:          exec,
		options:       options,
		threadOptions: threadOptions,
		id:            id,
	}
}

func (t *Thread) ID() (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.id, t.id != ""
}

func (t *Thread) setID(id string) {
	t.mu.Lock()
	t.id = id
	t.mu.Unlock()
}

func (t *Thread) RunStreamed(ctx context.Context, input Input, turnOptions *TurnOptions) (<-chan ThreadEvent, <-chan error) {
	events := make(chan ThreadEvent)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)

		if ctx == nil {
			ctx = context.Background()
		}
		if input == nil {
			errs <- errors.New("input cannot be nil")
			return
		}
		execCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		outputSchema, err := createOutputSchemaFile(turnOptions)
		if err != nil {
			errs <- err
			return
		}
		defer outputSchema.cleanup()

		prompt, images := input.normalizeCodexInput()
		t.mu.RLock()
		threadID := t.id
		t.mu.RUnlock()

		lines, runErrs := t.exec.run(execCtx, codexExecArgs{
			input:                 prompt,
			baseURL:               t.options.BaseURL,
			apiKey:                t.options.APIKey,
			threadID:              threadID,
			images:                images,
			model:                 t.threadOptions.Model,
			sandboxMode:           t.threadOptions.SandboxMode,
			workingDirectory:      t.threadOptions.WorkingDirectory,
			additionalDirectories: t.threadOptions.AdditionalDirectories,
			skipGitRepoCheck:      t.threadOptions.SkipGitRepoCheck,
			outputSchemaFile:      outputSchema.schemaPath,
			modelReasoningEffort:  t.threadOptions.ModelReasoningEffort,
			networkAccessEnabled:  t.threadOptions.NetworkAccessEnabled,
			webSearchMode:         t.threadOptions.WebSearchMode,
			webSearchEnabled:      t.threadOptions.WebSearchEnabled,
			approvalPolicy:        t.threadOptions.ApprovalPolicy,
		})

		for line := range lines {
			var event ThreadEvent
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				cancel()
				errs <- fmt.Errorf("failed to parse item: %s: %w", line, err)
				return
			}
			if event.Type == "thread.started" {
				t.setID(event.ThreadID)
			}

			select {
			case events <- event:
			case <-execCtx.Done():
				errs <- execCtx.Err()
				return
			}
		}

		if err := <-runErrs; err != nil {
			errs <- err
			return
		}
	}()

	return events, errs
}

func (t *Thread) Run(ctx context.Context, input Input, turnOptions *TurnOptions) (*Turn, error) {
	events, errs := t.RunStreamed(ctx, input, turnOptions)

	turn := &Turn{
		Items: make([]ThreadItem, 0),
	}
	var turnFailure *ThreadError

	for event := range events {
		switch event.Type {
		case "item.completed":
			if event.Item != nil {
				if item, ok := event.Item.(*AgentMessageItem); ok {
					turn.FinalResponse = item.Text
				}
				turn.Items = append(turn.Items, event.Item)
			}
		case "turn.completed":
			turn.Usage = event.Usage
		case "turn.failed":
			turnFailure = event.Error
		}
	}

	if err := <-errs; err != nil {
		return nil, err
	}
	if turnFailure != nil {
		return nil, errors.New(turnFailure.Message)
	}

	return turn, nil
}
