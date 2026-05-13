package codex

import "strings"

type Input interface {
	normalizeCodexInput() (prompt string, images []string)
}

type TextInput string

func (i TextInput) normalizeCodexInput() (string, []string) {
	return string(i), nil
}

type InputPart interface {
	inputPart()
}

type TextPart struct {
	Text string
}

func (TextPart) inputPart() {}

type LocalImagePart struct {
	Path string
}

func (LocalImagePart) inputPart() {}

type StructuredInput []InputPart

func (i StructuredInput) normalizeCodexInput() (string, []string) {
	promptParts := make([]string, 0, len(i))
	images := make([]string, 0)

	for _, part := range i {
		switch typed := part.(type) {
		case TextPart:
			promptParts = append(promptParts, typed.Text)
		case *TextPart:
			if typed != nil {
				promptParts = append(promptParts, typed.Text)
			}
		case LocalImagePart:
			images = append(images, typed.Path)
		case *LocalImagePart:
			if typed != nil {
				images = append(images, typed.Path)
			}
		}
	}

	return strings.Join(promptParts, "\n\n"), images
}
