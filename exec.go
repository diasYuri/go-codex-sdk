package codex

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

const (
	internalOriginatorEnv = "CODEX_INTERNAL_ORIGINATOR_OVERRIDE"
	goSDKOriginator       = "codex_sdk_go"
)

type codexExec struct {
	executablePath  string
	envOverride     map[string]string
	configOverrides CodexConfigObject
}

type codexExecArgs struct {
	input string

	baseURL               string
	apiKey                string
	threadID              string
	images                []string
	model                 string
	sandboxMode           SandboxMode
	workingDirectory      string
	additionalDirectories []string
	skipGitRepoCheck      bool
	outputSchemaFile      string
	modelReasoningEffort  ModelReasoningEffort
	networkAccessEnabled  *bool
	webSearchMode         WebSearchMode
	webSearchEnabled      *bool
	approvalPolicy        ApprovalMode
}

func newCodexExec(executablePath string, env map[string]string, configOverrides CodexConfigObject) *codexExec {
	return &codexExec{
		executablePath:  executablePath,
		envOverride:     cloneStringMap(env),
		configOverrides: cloneConfigObject(configOverrides),
	}
}

func (e *codexExec) run(ctx context.Context, args codexExecArgs) (<-chan string, <-chan error) {
	lines := make(chan string)
	errs := make(chan error, 1)

	go func() {
		defer close(lines)
		defer close(errs)

		commandArgs, err := e.buildCommandArgs(args)
		if err != nil {
			errs <- err
			return
		}

		cmd := exec.CommandContext(ctx, e.executablePath, commandArgs...)
		cmd.Env = e.buildEnv(args)

		stdin, err := cmd.StdinPipe()
		if err != nil {
			errs <- err
			return
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			errs <- err
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			errs <- err
			return
		}

		if err := cmd.Start(); err != nil {
			errs <- err
			return
		}

		var stderrBuffer bytes.Buffer
		stderrDone := make(chan struct{})
		go func() {
			_, _ = io.Copy(&stderrBuffer, stderr)
			close(stderrDone)
		}()

		stdinDone := make(chan error, 1)
		go func() {
			_, writeErr := io.WriteString(stdin, args.input)
			closeErr := stdin.Close()
			if writeErr != nil {
				stdinDone <- writeErr
				return
			}
			stdinDone <- closeErr
		}()

		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-ctx.Done():
				_ = cmd.Process.Kill()
				<-stderrDone
				_ = cmd.Wait()
				errs <- ctx.Err()
				return
			}
		}

		scanErr := scanner.Err()
		stdinErr := <-stdinDone
		waitErr := cmd.Wait()
		<-stderrDone

		if ctx.Err() != nil {
			errs <- ctx.Err()
			return
		}
		if scanErr != nil {
			errs <- scanErr
			return
		}
		if stdinErr != nil {
			errs <- stdinErr
			return
		}
		if waitErr != nil {
			errs <- fmt.Errorf("codex exec exited with %s: %s", exitDetail(waitErr), stderrBuffer.String())
			return
		}

		errs <- nil
	}()

	return lines, errs
}

func (e *codexExec) buildCommandArgs(args codexExecArgs) ([]string, error) {
	commandArgs := []string{"exec", "--experimental-json"}

	if e.configOverrides != nil {
		overrides, err := serializeConfigOverrides(e.configOverrides)
		if err != nil {
			return nil, err
		}
		for _, override := range overrides {
			commandArgs = append(commandArgs, "--config", override)
		}
	}

	if args.baseURL != "" {
		value, err := toTomlValue(args.baseURL, "openai_base_url")
		if err != nil {
			return nil, err
		}
		commandArgs = append(commandArgs, "--config", "openai_base_url="+value)
	}
	if args.model != "" {
		commandArgs = append(commandArgs, "--model", args.model)
	}
	if args.sandboxMode != "" {
		commandArgs = append(commandArgs, "--sandbox", string(args.sandboxMode))
	}
	if args.workingDirectory != "" {
		commandArgs = append(commandArgs, "--cd", args.workingDirectory)
	}
	for _, dir := range args.additionalDirectories {
		commandArgs = append(commandArgs, "--add-dir", dir)
	}
	if args.skipGitRepoCheck {
		commandArgs = append(commandArgs, "--skip-git-repo-check")
	}
	if args.outputSchemaFile != "" {
		commandArgs = append(commandArgs, "--output-schema", args.outputSchemaFile)
	}
	if args.modelReasoningEffort != "" {
		commandArgs = append(commandArgs, "--config", fmt.Sprintf("model_reasoning_effort=%q", args.modelReasoningEffort))
	}
	if args.networkAccessEnabled != nil {
		commandArgs = append(commandArgs, "--config", fmt.Sprintf("sandbox_workspace_write.network_access=%t", *args.networkAccessEnabled))
	}
	if args.webSearchMode != "" {
		commandArgs = append(commandArgs, "--config", fmt.Sprintf("web_search=%q", args.webSearchMode))
	} else if args.webSearchEnabled != nil {
		webSearchMode := WebSearchModeDisabled
		if *args.webSearchEnabled {
			webSearchMode = WebSearchModeLive
		}
		commandArgs = append(commandArgs, "--config", fmt.Sprintf("web_search=%q", webSearchMode))
	}
	if args.approvalPolicy != "" {
		commandArgs = append(commandArgs, "--config", fmt.Sprintf("approval_policy=%q", args.approvalPolicy))
	}
	if args.threadID != "" {
		commandArgs = append(commandArgs, "resume", args.threadID)
	}
	for _, image := range args.images {
		commandArgs = append(commandArgs, "--image", image)
	}

	return commandArgs, nil
}

func (e *codexExec) buildEnv(args codexExecArgs) []string {
	env := make(map[string]string)
	if e.envOverride != nil {
		for key, value := range e.envOverride {
			env[key] = value
		}
	} else {
		for _, item := range os.Environ() {
			key, value, ok := strings.Cut(item, "=")
			if ok {
				env[key] = value
			}
		}
	}

	if _, ok := env[internalOriginatorEnv]; !ok {
		env[internalOriginatorEnv] = goSDKOriginator
	}
	if args.apiKey != "" {
		env["CODEX_API_KEY"] = args.apiKey
	}

	out := make([]string, 0, len(env))
	for key, value := range env {
		out = append(out, key+"="+value)
	}
	return out
}

func exitDetail(err error) string {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status := exitErr.ProcessState; status != nil {
			return fmt.Sprintf("code %d", status.ExitCode())
		}
	}
	return err.Error()
}
