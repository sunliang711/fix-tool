package shell

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"fix-tool/internal/admin"
	"fix-tool/internal/fixsession"
	"fix-tool/internal/order"
	"fix-tool/internal/render"
	"fix-tool/internal/trace"
)

const (
	defaultStopTimeout = 5 * time.Second
	maxLineSize        = 1024 * 1024
)

var errShellExit = errors.New("shell exit")

type AdminService interface {
	Logon(context.Context) (admin.Result, error)
	Logout(context.Context) (admin.Result, error)
	Heartbeat(context.Context) (admin.Result, error)
	TestRequest(context.Context, string) (admin.Result, error)
}

type OrderService interface {
	NewOrder(context.Context, order.NewRequest) (order.Result, error)
	CancelOrder(context.Context, order.CancelRequest) (order.Result, error)
	ReplaceOrder(context.Context, order.ReplaceRequest) (order.Result, error)
}

type LineReader interface {
	ReadLine(context.Context, string) (string, bool, error)
	Close() error
}

type Options struct {
	In          io.Reader
	Out         io.Writer
	ErrOut      io.Writer
	LineReader  LineReader
	Admin       AdminService
	Order       OrderService
	Manager     fixsession.Manager
	Renderer    *render.Renderer
	Recorder    *trace.Recorder
	Transcript  *TranscriptRecorder
	Format      render.Format
	Prompt      string
	PromptCtl   PromptController
	StopTimeout time.Duration
}

type Runner struct {
	in          io.Reader
	out         io.Writer
	errOut      io.Writer
	lineReader  LineReader
	admin       AdminService
	order       OrderService
	manager     fixsession.Manager
	renderer    *render.Renderer
	recorder    *trace.Recorder
	transcript  *TranscriptRecorder
	format      render.Format
	prompt      string
	promptCtl   PromptController
	stopTimeout time.Duration
}

type scanResult struct {
	line string
	err  error
}

func NewRunner(options Options) *Runner {
	in := options.In
	if in == nil {
		in = os.Stdin
	}
	out := options.Out
	if out == nil {
		out = os.Stdout
	}
	errOut := options.ErrOut
	if errOut == nil {
		errOut = os.Stderr
	}
	renderer := options.Renderer
	if renderer == nil {
		renderer = render.NewRenderer(nil, render.Options{Format: options.Format})
	}
	recorder := options.Recorder
	if recorder == nil {
		recorder = trace.NewRecorder()
	}
	stopTimeout := options.StopTimeout
	if stopTimeout <= 0 {
		stopTimeout = defaultStopTimeout
	}
	return &Runner{
		in:          in,
		out:         out,
		errOut:      errOut,
		lineReader:  options.LineReader,
		admin:       options.Admin,
		order:       options.Order,
		manager:     options.Manager,
		renderer:    renderer,
		recorder:    recorder,
		transcript:  options.Transcript,
		format:      options.Format,
		prompt:      options.Prompt,
		promptCtl:   options.PromptCtl,
		stopTimeout: stopTimeout,
	}
}

func (r *Runner) Run(ctx context.Context) (err error) {
	defer func() {
		if stopErr := r.stopSession(); stopErr != nil && err == nil {
			err = stopErr
		}
		if closeErr := r.closeTranscript(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if err := ctx.Err(); err != nil {
		return err
	}
	if r.lineReader != nil {
		return r.runLineReader(ctx)
	}
	readCtx, cancelRead := context.WithCancel(ctx)
	results, readDone := r.startScan(readCtx)
	defer r.stopScan(cancelRead, readDone)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if r.prompt != "" {
			if _, err := fmt.Fprint(r.out, r.prompt); err != nil {
				return err
			}
			r.setPromptActive(true)
		}
		result, ok, err := r.nextScanResult(ctx, results, readDone)
		r.setPromptActive(false)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if err := r.handleLine(ctx, result.line); errors.Is(err, errShellExit) {
			return nil
		} else if err != nil {
			return err
		}
	}
}

func (r *Runner) runLineReader(ctx context.Context) (err error) {
	defer func() {
		if closeErr := r.lineReader.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, ok, err := r.lineReader.ReadLine(ctx, r.prompt)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if err := r.handleLine(ctx, line); errors.Is(err, errShellExit) {
			return nil
		} else if err != nil {
			return err
		}
	}
}

func (r *Runner) handleLine(ctx context.Context, line string) error {
	line = strings.TrimSpace(line)
	if len(line) == 0 {
		return nil
	}
	if err := r.recordInput(line); err != nil {
		return err
	}
	command, err := Parse(line)
	if err != nil {
		r.printCommandError(err)
		return nil
	}
	if command.Kind == CommandExit {
		return errShellExit
	}
	if command.Kind == CommandHelp {
		return r.renderHelp()
	}
	if err := r.execute(ctx, command); err != nil {
		r.printCommandError(err)
		return nil
	}
	return nil
}

func (r *Runner) renderHelp() error {
	_, err := fmt.Fprint(r.out, shellHelpText)
	return err
}

const shellHelpText = `Commands:
  help, ?                                      Show this help
  logon                                        Log on and keep the shell session
  logout                                       Send Logout
  heartbeat                                    Send Heartbeat
  test-request --id <id>                       Send TestRequest
  order new --symbol <s> --side <buy|sell> --qty <q> [--cl-ord-id <id>] [--price <p>] [--ord-type <type>] [--time-in-force <tif>] [--tag <tag=value>]
  order cancel --orig-cl-ord-id <id> --symbol <s> --side <buy|sell> [--cl-ord-id <id>] [--order-id <id>] [--tag <tag=value>]
  order replace --orig-cl-ord-id <id> --symbol <s> --side <buy|sell> --qty <q> --price <p> [--cl-ord-id <id>] [--order-id <id>] [--ord-type <type>] [--time-in-force <tif>] [--tag <tag=value>]
  save <file>                                  Start saving shell transcript
  save stop                                    Stop saving shell transcript
  save status                                  Show save status
  trace list                                   Show recorded traces
  exit                                         Exit shell

Keys:
  Up/Down                                      Browse command history
`

func (r *Runner) setPromptActive(active bool) {
	if r.promptCtl == nil {
		return
	}
	r.promptCtl.SetPromptActive(active)
}

func (r *Runner) startScan(ctx context.Context) (<-chan scanResult, <-chan struct{}) {
	results := make(chan scanResult)
	done := make(chan struct{})
	scanner := bufio.NewScanner(r.in)
	scanner.Buffer(make([]byte, 1024), maxLineSize)
	go func() {
		defer close(results)
		defer close(done)
		for scanner.Scan() {
			select {
			case results <- scanResult{line: scanner.Text()}:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case results <- scanResult{err: err}:
			case <-ctx.Done():
			}
		}
	}()
	return results, done
}

func (r *Runner) nextScanResult(ctx context.Context, results <-chan scanResult, readDone <-chan struct{}) (scanResult, bool, error) {
	select {
	case result, ok := <-results:
		if err := ctx.Err(); err != nil {
			r.interruptInput(readDone)
			return scanResult{}, false, err
		}
		if !ok {
			return scanResult{}, false, nil
		}
		if result.err != nil {
			return scanResult{}, false, result.err
		}
		return result, true, nil
	case <-ctx.Done():
		r.interruptInput(readDone)
		return scanResult{}, false, ctx.Err()
	}
}

func (r *Runner) stopScan(cancel context.CancelFunc, readDone <-chan struct{}) {
	cancel()
	r.interruptInput(readDone)
}

func (r *Runner) interruptInput(readDone <-chan struct{}) {
	select {
	case <-readDone:
		return
	default:
	}
	closer, ok := r.in.(io.Closer)
	if !ok {
		return
	}
	_ = closer.Close()
	<-readDone
}

func (r *Runner) execute(ctx context.Context, command Command) error {
	switch command.Kind {
	case CommandLogon:
		result, err := r.admin.Logon(ctx)
		if err != nil {
			return err
		}
		return r.recordAdminResult(result)
	case CommandLogout:
		result, err := r.admin.Logout(ctx)
		if err != nil {
			return err
		}
		return r.recordAdminResult(result)
	case CommandHeartbeat:
		result, err := r.admin.Heartbeat(ctx)
		if err != nil {
			return err
		}
		return r.recordAdminResult(result)
	case CommandTestRequest:
		result, err := r.admin.TestRequest(ctx, command.TestRequestID)
		if err != nil {
			return err
		}
		return r.recordAdminResult(result)
	case CommandOrderNew:
		result, err := r.order.NewOrder(ctx, command.NewRequest)
		if err != nil {
			return err
		}
		return r.renderOrderResult(result)
	case CommandOrderCancel:
		result, err := r.order.CancelOrder(ctx, command.CancelRequest)
		if err != nil {
			return err
		}
		return r.renderOrderResult(result)
	case CommandOrderReplace:
		result, err := r.order.ReplaceOrder(ctx, command.ReplaceRequest)
		if err != nil {
			return err
		}
		return r.renderOrderResult(result)
	case CommandTraceList:
		return r.renderTraceList()
	case CommandSaveStart:
		return r.startSave(command.SavePath)
	case CommandSaveStop:
		return r.stopSave()
	case CommandSaveStatus:
		return r.renderSaveStatus()
	default:
		return fmt.Errorf("unsupported command %q", command.Kind)
	}
}

func (r *Runner) recordInput(line string) error {
	if r.transcript == nil {
		return nil
	}
	return r.transcript.WriteInput(line)
}

func (r *Runner) startSave(path string) error {
	if r.transcript == nil {
		return fmt.Errorf("save is not available")
	}
	status, err := r.transcript.Start(path)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(r.out, "saving shell transcript to %s\n", status.Path)
	return err
}

func (r *Runner) stopSave() error {
	if r.transcript == nil {
		return fmt.Errorf("save is not available")
	}
	status := r.transcript.Status()
	if !status.Active {
		return fmt.Errorf("save is not active")
	}
	if _, err := fmt.Fprintf(r.out, "saving shell transcript stopped file=%s\n", status.Path); err != nil {
		return err
	}
	_, err := r.transcript.Stop()
	return err
}

func (r *Runner) renderSaveStatus() error {
	if r.transcript == nil {
		return fmt.Errorf("save is not available")
	}
	status := r.transcript.Status()
	if !status.Active {
		_, err := fmt.Fprintln(r.out, "save inactive")
		return err
	}
	_, err := fmt.Fprintf(r.out, "save active file=%s started_at=%s\n", status.Path, status.StartedAt.Format(time.RFC3339))
	return err
}

func (r *Runner) recordAdminResult(result admin.Result) error {
	r.record(result.Request)
	r.record(result.Response)
	return nil
}

func (r *Runner) renderOrderResult(result order.Result) error {
	if err := r.recordAndRender("Request", result.Request); err != nil {
		return err
	}
	return r.recordAndRender("Response", result.Response)
}

func (r *Runner) recordAndRender(title string, message *trace.MessageTrace) error {
	if message == nil {
		return nil
	}
	recorded := r.record(message)
	return r.renderTrace(title, recorded)
}

func (r *Runner) record(message *trace.MessageTrace) trace.MessageTrace {
	if message == nil {
		return trace.MessageTrace{}
	}
	return r.recorder.Record(*message)
}

func (r *Runner) renderTraceList() error {
	traces := r.recorder.List()
	if len(traces) == 0 {
		_, err := fmt.Fprintln(r.out, "no traces")
		return err
	}
	for i, message := range traces {
		if err := r.renderTrace(fmt.Sprintf("Trace %d", i+1), message); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) renderTrace(title string, message trace.MessageTrace) error {
	rendered, err := r.renderer.Render(message, r.format)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(r.out, "%s\n%s\n", title, rendered)
	return err
}

func (r *Runner) printCommandError(err error) {
	if err == nil {
		return
	}
	_, _ = fmt.Fprintf(r.errOut, "Error: %v\n", err)
}

func (r *Runner) stopSession() error {
	if r.manager == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.stopTimeout)
	defer cancel()
	return r.manager.Stop(ctx)
}

func (r *Runner) closeTranscript() error {
	if r.transcript == nil {
		return nil
	}
	return r.transcript.Close()
}
