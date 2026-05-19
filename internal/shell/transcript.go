package shell

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var errSaveInactive = errors.New("save is not active")

type TranscriptStatus struct {
	Active    bool
	Path      string
	StartedAt time.Time
}

type TranscriptRecorder struct {
	mu        sync.Mutex
	prompt    string
	file      *os.File
	path      string
	startedAt time.Time
}

func NewTranscriptRecorder(prompt string) *TranscriptRecorder {
	return &TranscriptRecorder{
		prompt: prompt,
	}
}

func (r *TranscriptRecorder) Wrap(writer io.Writer) io.Writer {
	return transcriptWriter{
		writer:   writer,
		recorder: r,
	}
}

func (r *TranscriptRecorder) Start(path string) (TranscriptStatus, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file != nil {
		return TranscriptStatus{}, fmt.Errorf("save already active: %s", r.path)
	}
	displayPath, err := filepath.Abs(path)
	if err != nil {
		displayPath = path
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsExist(err) {
			return TranscriptStatus{}, fmt.Errorf("save file already exists: %s", displayPath)
		}
		return TranscriptStatus{}, fmt.Errorf("open save file %s: %w", displayPath, err)
	}
	now := time.Now()
	r.file = file
	r.path = displayPath
	r.startedAt = now
	if _, err := fmt.Fprintf(file, "# fix-tool shell transcript\n# started_at=%s\n# file=%s\n\n", now.Format(time.RFC3339), displayPath); err != nil {
		_ = file.Close()
		r.file = nil
		r.path = ""
		r.startedAt = time.Time{}
		return TranscriptStatus{}, fmt.Errorf("write save header: %w", err)
	}
	return r.statusLocked(), nil
}

func (r *TranscriptRecorder) Stop() (TranscriptStatus, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return TranscriptStatus{}, errSaveInactive
	}
	status := r.statusLocked()
	if _, err := fmt.Fprintf(r.file, "\n# stopped_at=%s\n", time.Now().Format(time.RFC3339)); err != nil {
		return status, fmt.Errorf("write save footer: %w", err)
	}
	if err := r.file.Sync(); err != nil {
		return status, fmt.Errorf("sync save file: %w", err)
	}
	if err := r.file.Close(); err != nil {
		return status, fmt.Errorf("close save file: %w", err)
	}
	r.file = nil
	r.path = ""
	r.startedAt = time.Time{}
	return status, nil
}

func (r *TranscriptRecorder) Close() error {
	_, err := r.Stop()
	if errors.Is(err, errSaveInactive) {
		return nil
	}
	return err
}

func (r *TranscriptRecorder) Status() TranscriptStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.statusLocked()
}

func (r *TranscriptRecorder) WriteInput(line string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return nil
	}
	if _, err := fmt.Fprintf(r.file, "%s%s\n", r.prompt, line); err != nil {
		return fmt.Errorf("write save input: %w", err)
	}
	return nil
}

func (r *TranscriptRecorder) writeOutput(data []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return nil
	}
	if r.prompt != "" && string(data) == r.prompt {
		return nil
	}
	if _, err := r.file.Write(data); err != nil {
		return fmt.Errorf("write save output: %w", err)
	}
	return nil
}

func (r *TranscriptRecorder) statusLocked() TranscriptStatus {
	return TranscriptStatus{
		Active:    r.file != nil,
		Path:      r.path,
		StartedAt: r.startedAt,
	}
}

type transcriptWriter struct {
	writer   io.Writer
	recorder *TranscriptRecorder
}

func (w transcriptWriter) Write(data []byte) (int, error) {
	n := len(data)
	if w.writer != nil {
		written, err := w.writer.Write(data)
		n = written
		if err != nil {
			return n, err
		}
	}
	if w.recorder == nil || n == 0 {
		return n, nil
	}
	if err := w.recorder.writeOutput(data[:n]); err != nil {
		return n, err
	}
	return n, nil
}
