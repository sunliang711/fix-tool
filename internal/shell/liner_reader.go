package shell

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/peterh/liner"
)

type historyLineReader struct {
	state     *liner.State
	promptCtl PromptController
}

func NewHistoryLineReader(promptCtl PromptController) LineReader {
	state := liner.NewLiner()
	state.SetCtrlCAborts(true)
	return &historyLineReader{
		state:     state,
		promptCtl: promptCtl,
	}
}

func (r *historyLineReader) ReadLine(ctx context.Context, prompt string) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	r.setPromptActive(true)
	line, err := r.state.Prompt(prompt)
	r.setPromptActive(false)
	if errors.Is(err, io.EOF) {
		return "", false, nil
	}
	if errors.Is(err, liner.ErrPromptAborted) {
		return "", true, nil
	}
	if err != nil {
		return "", false, err
	}
	if strings.TrimSpace(line) != "" {
		r.state.AppendHistory(line)
	}
	return line, true, nil
}

func (r *historyLineReader) Close() error {
	return r.state.Close()
}

func (r *historyLineReader) setPromptActive(active bool) {
	if r.promptCtl == nil {
		return
	}
	r.promptCtl.SetPromptActive(active)
}
