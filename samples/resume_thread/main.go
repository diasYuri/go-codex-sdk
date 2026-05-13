package main

import (
	"context"
	"fmt"
	"log"
	"os"

	codex "github.com/diasYuri/go-codex-sdk"
)

func main() {
	threadID := os.Getenv("CODEX_THREAD_ID")
	if threadID == "" {
		log.Fatal("set CODEX_THREAD_ID to resume an existing thread")
	}

	client, err := codex.New(nil)
	if err != nil {
		log.Fatal(err)
	}

	thread := client.ResumeThread(threadID, nil)
	turn, err := thread.Run(context.Background(), codex.TextInput("Continue from the previous context"), nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(turn.FinalResponse)
}
