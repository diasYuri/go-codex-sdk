package codex

import "os/exec"

type Codex struct {
	exec    *codexExec
	options CodexOptions
}

func New(options *CodexOptions) (*Codex, error) {
	if options == nil {
		options = &CodexOptions{}
	}

	executablePath := options.CodexPathOverride
	if executablePath == "" {
		found, err := exec.LookPath("codex")
		if err != nil {
			return nil, err
		}
		executablePath = found
	}

	copied := *options
	copied.Env = cloneStringMap(options.Env)
	copied.Config = cloneConfigObject(options.Config)

	return &Codex{
		exec:    newCodexExec(executablePath, copied.Env, copied.Config),
		options: copied,
	}, nil
}

func (c *Codex) StartThread(options *ThreadOptions) *Thread {
	return newThread(c.exec, c.options, cloneThreadOptions(options), "")
}

func (c *Codex) ResumeThread(id string, options *ThreadOptions) *Thread {
	return newThread(c.exec, c.options, cloneThreadOptions(options), id)
}

func cloneStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneThreadOptions(input *ThreadOptions) ThreadOptions {
	if input == nil {
		return ThreadOptions{}
	}
	out := *input
	out.AdditionalDirectories = append([]string(nil), input.AdditionalDirectories...)
	return out
}
