package shell

import (
	"bytes"
	"fmt"
	"io"
	"sync"
)

type PromptController interface {
	SetPromptActive(active bool)
}

type PromptLogWriter struct {
	mu        sync.Mutex
	logOut    io.Writer
	promptOut io.Writer
	prompt    string
	active    bool
}

func NewPromptLogWriter(logOut io.Writer, promptOut io.Writer, prompt string) *PromptLogWriter {
	return &PromptLogWriter{
		logOut:    logOut,
		promptOut: promptOut,
		prompt:    prompt,
	}
}

func (w *PromptLogWriter) SetPromptActive(active bool) {
	if w == nil {
		return
	}
	w.mu.Lock()
	w.active = active
	w.mu.Unlock()
}

func (w *PromptLogWriter) Write(data []byte) (int, error) {
	if w == nil || w.logOut == nil {
		return len(data), nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.active || w.prompt == "" || w.promptOut == nil {
		n, err := w.logOut.Write(data)
		if n > len(data) {
			n = len(data)
		}
		return n, err
	}
	if _, err := fmt.Fprint(w.logOut, "\r\n"); err != nil {
		return 0, err
	}
	if _, err := w.logOut.Write(data); err != nil {
		return 0, err
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		if _, err := fmt.Fprint(w.logOut, "\n"); err != nil {
			return 0, err
		}
	}
	if _, err := fmt.Fprint(w.promptOut, w.prompt); err != nil {
		return 0, err
	}
	return len(data), nil
}
