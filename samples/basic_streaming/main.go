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
	events, errs := thread.RunStreamed(context.Background(), codex.TextInput("Diagnose the test failure"), nil)

	for event := range events {
		switch event.Type {
		case "item.completed":
			if event.Item != nil {
				fmt.Printf("item: %s (%s)\n", event.Item.GetID(), event.Item.GetType())
			}
		case "turn.completed":
			fmt.Printf("usage: %+v\n", event.Usage)
		}
	}

	if err := <-errs; err != nil {
		log.Fatal(err)
	}
}
