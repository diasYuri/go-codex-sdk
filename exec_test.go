package codex

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuildCommandArgsPreservesCompatibilityOrder(t *testing.T) {
	network := true
	webSearch := false
	exec := newCodexExec("codex", nil, CodexConfigObject{
		"approval_policy": "never",
	})

	args, err := exec.buildCommandArgs(codexExecArgs{
		baseURL:               "https://example.test",
		threadID:              "thread-id",
		images:                []string{"img.png"},
		model:                 "gpt-test-1",
		sandboxMode:           SandboxModeWorkspaceWrite,
		workingDirectory:      "/tmp/project",
		additionalDirectories: []string{"../backend", "/tmp/shared"},
		skipGitRepoCheck:      true,
		outputSchemaFile:      "/tmp/schema.json",
		modelReasoningEffort:  ModelReasoningEffortHigh,
		networkAccessEnabled:  &network,
		webSearchEnabled:      &webSearch,
		approvalPolicy:        ApprovalModeOnRequest,
	})
	if err != nil {
		t.Fatal(err)
	}

	assertPair(t, args, "--config", `approval_policy="never"`)
	assertPair(t, args, "--config", `openai_base_url="https://example.test"`)
	assertPair(t, args, "--model", "gpt-test-1")
	assertPair(t, args, "--sandbox", "workspace-write")
	assertPair(t, args, "--cd", "/tmp/project")
	assertPair(t, args, "--output-schema", "/tmp/schema.json")
	assertPair(t, args, "--config", `model_reasoning_effort="high"`)
	assertPair(t, args, "--config", "sandbox_workspace_write.network_access=true")
	assertPair(t, args, "--config", `web_search="disabled"`)
	assertPair(t, args, "--config", `approval_policy="on-request"`)

	resumeIndex := indexOf(args, "resume")
	imageIndex := indexOf(args, "--image")
	if resumeIndex == -1 || imageIndex == -1 || resumeIndex > imageIndex {
		t.Fatalf("resume must appear before image args: %#v", args)
	}

	addDirs := valuesAfter(args, "--add-dir")
	if !reflect.DeepEqual(addDirs, []string{"../backend", "/tmp/shared"}) {
		t.Fatalf("add dirs mismatch: %#v", addDirs)
	}
}

func TestBuildEnvOverrideDoesNotInheritProcessEnv(t *testing.T) {
	t.Setenv("CODEX_ENV_SHOULD_NOT_LEAK", "leak")
	exec := newCodexExec("codex", map[string]string{
		"CODEX_HOME": "/tmp/codex-home",
		"CUSTOM_ENV": "custom",
	}, nil)

	env := envSliceToMap(exec.buildEnv(codexExecArgs{apiKey: "test"}))
	if env["CODEX_HOME"] != "/tmp/codex-home" {
		t.Fatalf("CODEX_HOME mismatch: %#v", env)
	}
	if env["CUSTOM_ENV"] != "custom" {
		t.Fatalf("CUSTOM_ENV mismatch: %#v", env)
	}
	if _, ok := env["CODEX_ENV_SHOULD_NOT_LEAK"]; ok {
		t.Fatalf("env override leaked process env: %#v", env)
	}
	if env["CODEX_API_KEY"] != "test" {
		t.Fatalf("CODEX_API_KEY mismatch: %#v", env)
	}
	if env[internalOriginatorEnv] != goSDKOriginator {
		t.Fatalf("originator mismatch: %#v", env)
	}
}

func TestBuildEnvSetsPWDWhenWorkingDirectoryIsProvided(t *testing.T) {
	exec := newCodexExec("codex", map[string]string{
		"PWD":         "/tmp/old",
		"CODEX_HOME":  "/tmp/codex-home",
		"CUSTOM_ENV":  "custom",
		"EXTRA_VALUE": "extra",
	}, nil)

	env := envSliceToMap(exec.buildEnv(codexExecArgs{workingDirectory: "/tmp/project"}))
	if env["PWD"] != "/tmp/project" {
		t.Fatalf("PWD mismatch: %#v", env)
	}
	if env["CODEX_HOME"] != "/tmp/codex-home" {
		t.Fatalf("CODEX_HOME mismatch: %#v", env)
	}
	if env["CUSTOM_ENV"] != "custom" {
		t.Fatalf("CUSTOM_ENV mismatch: %#v", env)
	}
	if env["EXTRA_VALUE"] != "extra" {
		t.Fatalf("EXTRA_VALUE mismatch: %#v", env)
	}
	if env[internalOriginatorEnv] != goSDKOriginator {
		t.Fatalf("originator mismatch: %#v", env)
	}
}

func TestCodexExecRunStreamsLinesAndWritesInput(t *testing.T) {
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args.txt")
	stdinPath := filepath.Join(dir, "stdin.txt")
	fake := writeFakeCodex(t, dir)

	exec := newCodexExec(fake, map[string]string{
		"PATH":                  os.Getenv("PATH"),
		"FAKE_CODEX_ARGS_FILE":  argsPath,
		"FAKE_CODEX_STDIN_FILE": stdinPath,
		"FAKE_CODEX_STDOUT":     `{"type":"turn.started"}` + "\n" + `{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":2,"reasoning_output_tokens":0}}`,
	}, nil)

	lines, errs := exec.run(context.Background(), codexExecArgs{input: "hello"})
	var got []string
	for line := range lines {
		got = append(got, line)
	}
	if err := <-errs; err != nil {
		t.Fatal(err)
	}

	expectedLines := []string{
		`{"type":"turn.started"}`,
		`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":2,"reasoning_output_tokens":0}}`,
	}
	if !reflect.DeepEqual(got, expectedLines) {
		t.Fatalf("lines mismatch\nwant: %#v\n got: %#v", expectedLines, got)
	}
	if data := readFile(t, stdinPath); data != "hello" {
		t.Fatalf("stdin mismatch: %q", data)
	}
	if args := strings.Fields(readFile(t, argsPath)); !reflect.DeepEqual(args[:2], []string{"exec", "--experimental-json"}) {
		t.Fatalf("args mismatch: %#v", args)
	}
}

func TestCodexExecRunUsesWorkingDirectoryAsProcessCwd(t *testing.T) {
	workingDir := t.TempDir()
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args.txt")
	stdinPath := filepath.Join(dir, "stdin.txt")
	pwdPath := filepath.Join(dir, "pwd.txt")
	fake := writeFakeCodex(t, dir)

	exec := newCodexExec(fake, map[string]string{
		"PATH":                  os.Getenv("PATH"),
		"FAKE_CODEX_ARGS_FILE":  argsPath,
		"FAKE_CODEX_STDIN_FILE": stdinPath,
		"FAKE_CODEX_STDOUT":     `{"type":"turn.started"}` + "\n" + `{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":2,"reasoning_output_tokens":0}}`,
		"FAKE_CODEX_PWD_FILE":   pwdPath,
	}, nil)

	lines, errs := exec.run(context.Background(), codexExecArgs{
		input:            "hello",
		workingDirectory: workingDir,
	})
	var got []string
	for line := range lines {
		got = append(got, line)
	}
	if err := <-errs; err != nil {
		t.Fatal(err)
	}

	expectedLines := []string{
		`{"type":"turn.started"}`,
		`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":2,"reasoning_output_tokens":0}}`,
	}
	if !reflect.DeepEqual(got, expectedLines) {
		t.Fatalf("lines mismatch\nwant: %#v\n got: %#v", expectedLines, got)
	}
	if data := readFile(t, stdinPath); data != "hello" {
		t.Fatalf("stdin mismatch: %q", data)
	}
	if args := strings.Fields(readFile(t, argsPath)); !reflect.DeepEqual(args[:2], []string{"exec", "--experimental-json"}) {
		t.Fatalf("args mismatch: %#v", args)
	}
	assertPair(t, strings.Fields(readFile(t, argsPath)), "--cd", workingDir)
	if gotPwd := strings.TrimSpace(readFile(t, pwdPath)); gotPwd != workingDir {
		t.Fatalf("cwd mismatch\nwant: %q\n got: %q", workingDir, gotPwd)
	}
}

func TestCodexExecRunReturnsExitErrorWithStderr(t *testing.T) {
	dir := t.TempDir()
	fake := writeFakeCodex(t, dir)
	exec := newCodexExec(fake, map[string]string{
		"PATH":              os.Getenv("PATH"),
		"FAKE_CODEX_STDERR": "boom",
		"FAKE_CODEX_EXIT":   "2",
	}, nil)

	lines, errs := exec.run(context.Background(), codexExecArgs{input: "hello"})
	for range lines {
	}
	err := <-errs
	if err == nil || !strings.Contains(err.Error(), "code 2") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected exit error with stderr, got %v", err)
	}
}

func writeFakeCodex(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "codex-fake")
	script := `#!/bin/sh
if [ -n "$FAKE_CODEX_ARGS_FILE" ]; then
  printf '%s\n' "$@" > "$FAKE_CODEX_ARGS_FILE"
fi
if [ -n "$FAKE_CODEX_STDIN_FILE" ]; then
  cat > "$FAKE_CODEX_STDIN_FILE"
else
  cat > /dev/null
fi
if [ -n "$FAKE_CODEX_PWD_FILE" ]; then
  pwd > "$FAKE_CODEX_PWD_FILE"
fi
if [ -n "$FAKE_CODEX_STDERR" ]; then
  printf '%s' "$FAKE_CODEX_STDERR" >&2
fi
if [ -n "$FAKE_CODEX_STDOUT" ]; then
  printf '%s\n' "$FAKE_CODEX_STDOUT"
fi
if [ -n "$FAKE_CODEX_EXIT" ]; then
  exit "$FAKE_CODEX_EXIT"
fi
exit 0
`
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertPair(t *testing.T, args []string, key string, value string) {
	t.Helper()
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key && args[i+1] == value {
			return
		}
	}
	t.Fatalf("pair %s %s not found in %#v", key, value, args)
}

func indexOf(args []string, value string) int {
	for i, arg := range args {
		if arg == value {
			return i
		}
	}
	return -1
}

func valuesAfter(args []string, key string) []string {
	var values []string
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key {
			values = append(values, args[i+1])
		}
	}
	return values
}

func envSliceToMap(items []string) map[string]string {
	out := make(map[string]string, len(items))
	for _, item := range items {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			out[key] = value
		}
	}
	return out
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
