package codex

import (
	"encoding/json"
)

type CodexConfigValue any

type CodexConfigObject map[string]CodexConfigValue

type CodexOptions struct {
	CodexPathOverride string
	BaseURL           string
	APIKey            string
	Config            CodexConfigObject
	Env               map[string]string
}

type ApprovalMode string

const (
	ApprovalModeNever     ApprovalMode = "never"
	ApprovalModeOnRequest ApprovalMode = "on-request"
	ApprovalModeOnFailure ApprovalMode = "on-failure"
	ApprovalModeUntrusted ApprovalMode = "untrusted"
)

type SandboxMode string

const (
	SandboxModeReadOnly         SandboxMode = "read-only"
	SandboxModeWorkspaceWrite   SandboxMode = "workspace-write"
	SandboxModeDangerFullAccess SandboxMode = "danger-full-access"
)

type ModelReasoningEffort string

const (
	ModelReasoningEffortMinimal ModelReasoningEffort = "minimal"
	ModelReasoningEffortLow     ModelReasoningEffort = "low"
	ModelReasoningEffortMedium  ModelReasoningEffort = "medium"
	ModelReasoningEffortHigh    ModelReasoningEffort = "high"
	ModelReasoningEffortXHigh   ModelReasoningEffort = "xhigh"
)

type WebSearchMode string

const (
	WebSearchModeDisabled WebSearchMode = "disabled"
	WebSearchModeCached   WebSearchMode = "cached"
	WebSearchModeLive     WebSearchMode = "live"
)

type ThreadOptions struct {
	Model                 string
	SandboxMode           SandboxMode
	WorkingDirectory      string
	SkipGitRepoCheck      bool
	ModelReasoningEffort  ModelReasoningEffort
	NetworkAccessEnabled  *bool
	WebSearchMode         WebSearchMode
	WebSearchEnabled      *bool
	ApprovalPolicy        ApprovalMode
	AdditionalDirectories []string
}

type TurnOptions struct {
	OutputSchema any
}

type Usage struct {
	InputTokens           int `json:"input_tokens"`
	CachedInputTokens     int `json:"cached_input_tokens"`
	OutputTokens          int `json:"output_tokens"`
	ReasoningOutputTokens int `json:"reasoning_output_tokens"`
}

type ThreadError struct {
	Message string `json:"message"`
}

type ThreadEvent struct {
	Type     string       `json:"type"`
	ThreadID string       `json:"thread_id,omitempty"`
	Usage    *Usage       `json:"usage,omitempty"`
	Item     ThreadItem   `json:"item,omitempty"`
	Error    *ThreadError `json:"error,omitempty"`
	Message  string       `json:"message,omitempty"`
}

func (e *ThreadEvent) UnmarshalJSON(data []byte) error {
	type eventAlias struct {
		Type     string          `json:"type"`
		ThreadID string          `json:"thread_id"`
		Usage    *Usage          `json:"usage"`
		Item     json.RawMessage `json:"item"`
		Error    *ThreadError    `json:"error"`
		Message  string          `json:"message"`
	}

	var raw eventAlias
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	e.Type = raw.Type
	e.ThreadID = raw.ThreadID
	e.Usage = raw.Usage
	e.Error = raw.Error
	e.Message = raw.Message
	e.Item = nil

	if len(raw.Item) != 0 && string(raw.Item) != "null" {
		item, err := unmarshalThreadItem(raw.Item)
		if err != nil {
			return err
		}
		e.Item = item
	}

	return nil
}

type ThreadItem interface {
	threadItem()
	GetID() string
	GetType() string
}

type itemHeader struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

func unmarshalThreadItem(data []byte) (ThreadItem, error) {
	var header itemHeader
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, err
	}

	switch header.Type {
	case "agent_message":
		var item AgentMessageItem
		return &item, json.Unmarshal(data, &item)
	case "reasoning":
		var item ReasoningItem
		return &item, json.Unmarshal(data, &item)
	case "command_execution":
		var item CommandExecutionItem
		return &item, json.Unmarshal(data, &item)
	case "file_change":
		var item FileChangeItem
		return &item, json.Unmarshal(data, &item)
	case "mcp_tool_call":
		var item McpToolCallItem
		return &item, json.Unmarshal(data, &item)
	case "web_search":
		var item WebSearchItem
		return &item, json.Unmarshal(data, &item)
	case "todo_list":
		var item TodoListItem
		return &item, json.Unmarshal(data, &item)
	case "error":
		var item ErrorItem
		return &item, json.Unmarshal(data, &item)
	default:
		return &UnknownItem{ID: header.ID, Type: header.Type, Raw: append(json.RawMessage(nil), data...)}, nil
	}
}

type CommandExecutionStatus string

const (
	CommandExecutionStatusInProgress CommandExecutionStatus = "in_progress"
	CommandExecutionStatusCompleted  CommandExecutionStatus = "completed"
	CommandExecutionStatusFailed     CommandExecutionStatus = "failed"
)

type CommandExecutionItem struct {
	ID               string                 `json:"id"`
	Type             string                 `json:"type"`
	Command          string                 `json:"command"`
	AggregatedOutput string                 `json:"aggregated_output"`
	ExitCode         *int                   `json:"exit_code,omitempty"`
	Status           CommandExecutionStatus `json:"status"`
}

func (*CommandExecutionItem) threadItem()       {}
func (i *CommandExecutionItem) GetID() string   { return i.ID }
func (i *CommandExecutionItem) GetType() string { return i.Type }

type PatchChangeKind string

const (
	PatchChangeKindAdd    PatchChangeKind = "add"
	PatchChangeKindDelete PatchChangeKind = "delete"
	PatchChangeKindUpdate PatchChangeKind = "update"
)

type FileUpdateChange struct {
	Path string          `json:"path"`
	Kind PatchChangeKind `json:"kind"`
}

type PatchApplyStatus string

const (
	PatchApplyStatusCompleted PatchApplyStatus = "completed"
	PatchApplyStatusFailed    PatchApplyStatus = "failed"
)

type FileChangeItem struct {
	ID      string             `json:"id"`
	Type    string             `json:"type"`
	Changes []FileUpdateChange `json:"changes"`
	Status  PatchApplyStatus   `json:"status"`
}

func (*FileChangeItem) threadItem()       {}
func (i *FileChangeItem) GetID() string   { return i.ID }
func (i *FileChangeItem) GetType() string { return i.Type }

type McpToolCallStatus string

const (
	McpToolCallStatusInProgress McpToolCallStatus = "in_progress"
	McpToolCallStatusCompleted  McpToolCallStatus = "completed"
	McpToolCallStatusFailed     McpToolCallStatus = "failed"
)

type McpToolCallResult struct {
	Content           []any `json:"content"`
	StructuredContent any   `json:"structured_content"`
}

type McpToolCallError struct {
	Message string `json:"message"`
}

type McpToolCallItem struct {
	ID        string             `json:"id"`
	Type      string             `json:"type"`
	Server    string             `json:"server"`
	Tool      string             `json:"tool"`
	Arguments any                `json:"arguments"`
	Result    *McpToolCallResult `json:"result,omitempty"`
	Error     *McpToolCallError  `json:"error,omitempty"`
	Status    McpToolCallStatus  `json:"status"`
}

func (*McpToolCallItem) threadItem()       {}
func (i *McpToolCallItem) GetID() string   { return i.ID }
func (i *McpToolCallItem) GetType() string { return i.Type }

type AgentMessageItem struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Text string `json:"text"`
}

func (*AgentMessageItem) threadItem()       {}
func (i *AgentMessageItem) GetID() string   { return i.ID }
func (i *AgentMessageItem) GetType() string { return i.Type }

type ReasoningItem struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Text string `json:"text"`
}

func (*ReasoningItem) threadItem()       {}
func (i *ReasoningItem) GetID() string   { return i.ID }
func (i *ReasoningItem) GetType() string { return i.Type }

type WebSearchItem struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Query string `json:"query"`
}

func (*WebSearchItem) threadItem()       {}
func (i *WebSearchItem) GetID() string   { return i.ID }
func (i *WebSearchItem) GetType() string { return i.Type }

type ErrorItem struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (*ErrorItem) threadItem()       {}
func (i *ErrorItem) GetID() string   { return i.ID }
func (i *ErrorItem) GetType() string { return i.Type }

type TodoItem struct {
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
}

type TodoListItem struct {
	ID    string     `json:"id"`
	Type  string     `json:"type"`
	Items []TodoItem `json:"items"`
}

func (*TodoListItem) threadItem()       {}
func (i *TodoListItem) GetID() string   { return i.ID }
func (i *TodoListItem) GetType() string { return i.Type }

type UnknownItem struct {
	ID   string
	Type string
	Raw  json.RawMessage
}

func (*UnknownItem) threadItem()       {}
func (i *UnknownItem) GetID() string   { return i.ID }
func (i *UnknownItem) GetType() string { return i.Type }
