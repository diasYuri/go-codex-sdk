package main

import (
	"context"
	"fmt"
	"log"

	codex "github.com/diasYuri/go-codex-sdk"
)

func main() {
	client, err := codex.New(nil)
	if err != nil {
		log.Fatal(err)
	}

	thread := client.StartThread(nil)
	turn, err := thread.Run(context.Background(), codex.TextInput("Summarize the repository status"), &codex.TurnOptions{
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"summary": map[string]any{"type": "string"},
				"status": map[string]any{
					"type": "string",
					"enum": []string{"ok", "action_required"},
				},
			},
			"required":             []string{"summary", "status"},
			"additionalProperties": false,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(turn.FinalResponse)
}
