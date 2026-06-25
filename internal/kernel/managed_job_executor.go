package kernel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	managedJobOutputSnapshotMinBytes    = 1024
	managedJobOutputSnapshotMinInterval = 2 * time.Second
	managedJobOutputSnapshotMaxEvents   = 8
)

type ManagedJobExecutor interface {
	Start(ctx context.Context, request ManagedJobStartRequest) error
	Cancel(jobID string, reason string) (bool, error)
}

type ManagedJobExecutorCapabilities struct {
	ForegroundAttach bool
}

type managedJobExecutorCapabilityReporter interface {
	Capabilities() ManagedJobExecutorCapabilities
}

type managedJobForegroundAttachExecutor interface {
	AttachForeground(ctx context.Context, request ManagedJobForegroundAttachRequest) (ManagedJobForegroundAttachResult, error)
}

type managedShellForegroundRunner interface {
	RunForeground(ctx context.Context, request ManagedShellForegroundRequest) (ManagedShellForegroundResult, error)
}

type managedShellForegroundKiller interface {
	KillForeground(operationID string, reason string) bool
}

type ManagedShellForegroundRequest struct {
	OperationID string
	CWD         string
	Command     string
	Timeout     time.Duration
}

type ManagedShellForegroundResult struct {
	Stdout      capturedOutput
	Stderr      capturedOutput
	ExitCode    int
	TimedOut    bool
	Interrupted bool
}

type ManagedJobForegroundAttachRequest struct {
	SessionID     string
	TurnID        string
	OperationID   string
	Job           JobProjection
	Reason        string
	InterruptedAt time.Time
	Observe       func(JobProjection)
	Complete      func(JobProjection)
}

type ManagedJobForegroundAttachResult struct {
	Attached      bool
	FailureReason string
}

type ManagedJobStartRequest struct {
	Job      JobProjection
	Observe  func(JobProjection)
	Complete func(JobProjection)
}

type localManagedJobExecutor struct {
	root   context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu         sync.Mutex
	running    map[string]*localManagedShellProcess
	foreground map[string]*localManagedShellProcess

	beforeProcessStart func()
}

type localManagedShellProcess struct {
	processCtx context.Context
	cancel     context.CancelFunc
	output     *managedJobOutputCapture
	done       chan localManagedShellProcessResult

	mu     sync.Mutex
	reason string
}

type localManagedShellProcessResult struct {
	stdout   capturedOutput
	stderr   capturedOutput
	exitCode int
	err      error
	ctxErr   error
}

func newLocalManagedJobExecutor() *localManagedJobExecutor {
	root, cancel := context.WithCancel(context.Background())
	return &localManagedJobExecutor{
		root:       root,
		cancel:     cancel,
		running:    map[string]*localManagedShellProcess{},
		foreground: map[string]*localManagedShellProcess{},
	}
}

func (e *localManagedJobExecutor) Start(_ context.Context, request ManagedJobStartRequest) error {
	job := request.Job
	jobID := strings.TrimSpace(job.JobID)
	if jobID == "" {
		return fmt.Errorf("job id is required")
	}
	if request.Complete == nil {
		return fmt.Errorf("job completion callback is required")
	}
	select {
	case <-e.root.Done():
		return fmt.Errorf("job executor is closed")
	default:
	}
	e.mu.Lock()
	if _, exists := e.running[jobID]; exists {
		e.mu.Unlock()
		return fmt.Errorf("job %s is already running", jobID)
	}
	e.mu.Unlock()

	active, cmd, err := e.newManagedShellProcess(job.CWD, job.Command, request.Observe)
	if err != nil {
		return err
	}
	e.mu.Lock()
	if _, exists := e.running[jobID]; exists {
		e.mu.Unlock()
		return fmt.Errorf("job %s is already running", jobID)
	}
	e.running[jobID] = active
	e.mu.Unlock()
	if err := e.startReservedManagedShellProcess(active, cmd); err != nil {
		e.mu.Lock()
		if e.running[jobID] == active {
			delete(e.running, jobID)
		}
		e.mu.Unlock()
		active.cancelWithReason("process_start_failed")
		return err
	}

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer func() {
			e.mu.Lock()
			if e.running[jobID] == active {
				delete(e.running, jobID)
			}
			e.mu.Unlock()
		}()
		result := <-active.done
		finished := jobFromManagedProcessResult(job, result, active.cancelReason())
		request.Complete(finished)
	}()
	return nil
}

func (e *localManagedJobExecutor) Capabilities() ManagedJobExecutorCapabilities {
	return ManagedJobExecutorCapabilities{ForegroundAttach: true}
}

func (e *localManagedJobExecutor) RunForeground(ctx context.Context, request ManagedShellForegroundRequest) (ManagedShellForegroundResult, error) {
	operationID := strings.TrimSpace(request.OperationID)
	if operationID == "" {
		return ManagedShellForegroundResult{}, fmt.Errorf("operation id is required")
	}
	if request.Timeout <= 0 {
		request.Timeout = defaultShellDuration
	}
	if ctx == nil {
		ctx = context.Background()
	}
	active, cmd, err := e.newManagedShellProcess(request.CWD, request.Command, nil)
	if err != nil {
		return ManagedShellForegroundResult{}, err
	}
	e.mu.Lock()
	if _, exists := e.foreground[operationID]; exists {
		e.mu.Unlock()
		return ManagedShellForegroundResult{}, fmt.Errorf("foreground operation %s is already running", operationID)
	}
	e.foreground[operationID] = active
	e.mu.Unlock()
	if err := e.startReservedManagedShellProcess(active, cmd); err != nil {
		e.removeForeground(operationID, active)
		active.cancelWithReason("process_start_failed")
		return ManagedShellForegroundResult{}, err
	}

	timer := time.NewTimer(request.Timeout)
	defer timer.Stop()
	select {
	case result := <-active.done:
		e.removeForeground(operationID, active)
		return ManagedShellForegroundResult{
			Stdout:   result.stdout,
			Stderr:   result.stderr,
			ExitCode: result.exitCode,
		}, result.err
	case <-timer.C:
		active.cancelWithReason(foregroundTimeoutReason)
		result := <-active.done
		e.removeForeground(operationID, active)
		return ManagedShellForegroundResult{
			Stdout:   result.stdout,
			Stderr:   result.stderr,
			ExitCode: foregroundTimeoutExitCode,
			TimedOut: true,
		}, nil
	case <-ctx.Done():
		return ManagedShellForegroundResult{Interrupted: true}, nil
	case <-e.root.Done():
		active.cancelWithReason("executor_closed")
		result := <-active.done
		e.removeForeground(operationID, active)
		return ManagedShellForegroundResult{
			Stdout:   result.stdout,
			Stderr:   result.stderr,
			ExitCode: result.exitCode,
		}, e.root.Err()
	}
}

func (e *localManagedJobExecutor) AttachForeground(_ context.Context, request ManagedJobForegroundAttachRequest) (ManagedJobForegroundAttachResult, error) {
	operationID := strings.TrimSpace(request.OperationID)
	jobID := strings.TrimSpace(request.Job.JobID)
	if operationID == "" || jobID == "" {
		return ManagedJobForegroundAttachResult{Attached: false, FailureReason: "missing_identity"}, nil
	}
	if request.Complete == nil {
		return ManagedJobForegroundAttachResult{}, fmt.Errorf("job completion callback is required")
	}
	e.mu.Lock()
	active := e.foreground[operationID]
	if active == nil {
		e.mu.Unlock()
		return ManagedJobForegroundAttachResult{Attached: false, FailureReason: "foreground_process_not_found"}, nil
	}
	if _, exists := e.running[jobID]; exists {
		e.mu.Unlock()
		return ManagedJobForegroundAttachResult{Attached: false, FailureReason: "job_already_running"}, nil
	}
	delete(e.foreground, operationID)
	e.running[jobID] = active
	active.output.setObserver(request.Observe)
	e.mu.Unlock()

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer func() {
			e.mu.Lock()
			if e.running[jobID] == active {
				delete(e.running, jobID)
			}
			e.mu.Unlock()
		}()
		result := <-active.done
		finished := jobFromManagedProcessResult(request.Job, result, active.cancelReason())
		request.Complete(finished)
	}()
	return ManagedJobForegroundAttachResult{Attached: true}, nil
}

func (e *localManagedJobExecutor) Cancel(jobID string, reason string) (bool, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return false, nil
	}
	e.mu.Lock()
	active := e.running[jobID]
	if active == nil {
		e.mu.Unlock()
		return false, nil
	}
	e.mu.Unlock()
	active.cancelWithReason(reason)
	return true, nil
}

func (e *localManagedJobExecutor) KillForeground(operationID string, reason string) bool {
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return false
	}
	e.mu.Lock()
	active := e.foreground[operationID]
	if active != nil {
		delete(e.foreground, operationID)
	}
	e.mu.Unlock()
	if active == nil {
		return false
	}
	active.cancelWithReason(reason)
	return true
}

func managedJobExecutorCapabilities(executor ManagedJobExecutor) ManagedJobExecutorCapabilities {
	if executor == nil {
		return ManagedJobExecutorCapabilities{}
	}
	reporter, ok := executor.(managedJobExecutorCapabilityReporter)
	if !ok {
		return ManagedJobExecutorCapabilities{}
	}
	capabilities := reporter.Capabilities()
	if capabilities.ForegroundAttach {
		if _, ok := executor.(managedJobForegroundAttachExecutor); !ok {
			capabilities.ForegroundAttach = false
		}
		if _, ok := executor.(managedShellForegroundRunner); !ok {
			capabilities.ForegroundAttach = false
		}
		if _, ok := executor.(managedShellForegroundKiller); !ok {
			capabilities.ForegroundAttach = false
		}
	}
	return capabilities
}

func (e *localManagedJobExecutor) Close() {
	e.cancel()
	e.wg.Wait()
}

func (e *localManagedJobExecutor) cancelReason(jobID string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	active := e.running[strings.TrimSpace(jobID)]
	if active == nil {
		return ""
	}
	return active.reason
}

func (e *localManagedJobExecutor) newManagedShellProcess(cwd string, command string, observe func(JobProjection)) (*localManagedShellProcess, *exec.Cmd, error) {
	select {
	case <-e.root.Done():
		return nil, nil, fmt.Errorf("job executor is closed")
	default:
	}
	processCtx, cancel := context.WithCancel(e.root)
	output := newManagedJobOutputCapture(observe)
	cmd := platformShellCommand(processCtx, command)
	cmd.Dir = cwd
	cmd.Env = shellProcessEnvironment()
	configureShellProcessTermination(cmd)
	cmd.WaitDelay = 2 * time.Second
	cmd.Stdout = output.stdoutWriter()
	cmd.Stderr = output.stderrWriter()
	active := &localManagedShellProcess{
		processCtx: processCtx,
		cancel:     cancel,
		output:     output,
		done:       make(chan localManagedShellProcessResult, 1),
	}
	return active, cmd, nil
}

func (e *localManagedJobExecutor) startReservedManagedShellProcess(active *localManagedShellProcess, cmd *exec.Cmd) error {
	if active == nil || cmd == nil {
		return fmt.Errorf("managed shell process is required")
	}
	if e.beforeProcessStart != nil {
		e.beforeProcessStart()
	}
	if err := cmd.Start(); err != nil {
		active.cancelWithReason("process_start_failed")
		return err
	}
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		err := cmd.Wait()
		exitCode := 0
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				exitCode = exitErr.ExitCode()
				err = nil
			}
		}
		stdout, stderr := active.output.capture()
		active.done <- localManagedShellProcessResult{
			stdout:   stdout,
			stderr:   stderr,
			exitCode: exitCode,
			err:      err,
			ctxErr:   active.processCtx.Err(),
		}
		close(active.done)
	}()
	return nil
}

func (e *localManagedJobExecutor) removeForeground(operationID string, active *localManagedShellProcess) {
	e.mu.Lock()
	if e.foreground[strings.TrimSpace(operationID)] == active {
		delete(e.foreground, strings.TrimSpace(operationID))
	}
	e.mu.Unlock()
}

func (p *localManagedShellProcess) cancelWithReason(reason string) {
	p.mu.Lock()
	p.reason = strings.TrimSpace(reason)
	cancel := p.cancel
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (p *localManagedShellProcess) cancelReason() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.reason
}

func jobFromManagedProcessResult(job JobProjection, result localManagedShellProcessResult, cancelReason string) JobProjection {
	finished := job
	if result.err == nil {
		finished.ExitCode = &result.exitCode
	}
	applyJobOutputCapture(&finished, result.stdout, result.stderr)
	switch {
	case result.ctxErr != nil:
		finished.Status = "cancelled"
		finished.CancelReason = strings.TrimSpace(cancelReason)
	case result.err != nil:
		finished.Status = "failed"
		finished.FailureReason = "shell_runtime_failed: " + result.err.Error()
	case result.exitCode == 0:
		finished.Status = "completed"
	default:
		finished.Status = "failed"
	}
	return finished
}

type managedJobOutputCapture struct {
	mu sync.Mutex

	stdout cappedBuffer
	stderr cappedBuffer

	observe            func(JobProjection)
	emitted            bool
	stopped            bool
	snapshotCount      int
	bytesSinceSnapshot int
	lastSnapshot       time.Time
	now                func() time.Time
}

func newManagedJobOutputCapture(observe func(JobProjection)) *managedJobOutputCapture {
	capture := &managedJobOutputCapture{
		observe: observe,
		now:     time.Now,
	}
	capture.stdout.limit = maxShellOutputBytes
	capture.stderr.limit = maxShellOutputBytes
	return capture
}

func (c *managedJobOutputCapture) setObserver(observe func(JobProjection)) {
	if c == nil {
		return
	}
	var snapshot JobProjection
	shouldObserve := false
	c.mu.Lock()
	c.observe = observe
	if observe != nil && !c.stopped {
		snapshot = c.snapshotLocked()
		if strings.TrimSpace(snapshot.Stdout) != "" || strings.TrimSpace(snapshot.Stderr) != "" || snapshot.StdoutTruncated || snapshot.StderrTruncated {
			c.emitted = true
			c.snapshotCount++
			c.bytesSinceSnapshot = 0
			c.lastSnapshot = c.now()
			shouldObserve = true
		}
	}
	c.mu.Unlock()
	if shouldObserve {
		observe(snapshot)
	}
}

func (c *managedJobOutputCapture) stdoutWriter() io.Writer {
	return managedJobOutputStreamWriter{capture: c, stream: "stdout"}
}

func (c *managedJobOutputCapture) stderrWriter() io.Writer {
	return managedJobOutputStreamWriter{capture: c, stream: "stderr"}
}

func (c *managedJobOutputCapture) capture() (capturedOutput, capturedOutput) {
	if c == nil {
		return capturedOutput{}, capturedOutput{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stdout.Capture(), c.stderr.Capture()
}

func (c *managedJobOutputCapture) write(stream string, data []byte) (int, error) {
	if c == nil {
		return len(data), nil
	}
	written := len(data)
	if written == 0 {
		return 0, nil
	}

	var snapshot JobProjection
	shouldObserve := false
	c.mu.Lock()
	switch stream {
	case "stderr":
		_, _ = c.stderr.Write(data)
	default:
		_, _ = c.stdout.Write(data)
	}
	c.bytesSinceSnapshot += written
	now := c.now()
	if c.shouldObserveLocked(now) {
		snapshot = c.snapshotLocked()
		c.emitted = true
		c.snapshotCount++
		if c.snapshotCount >= managedJobOutputSnapshotMaxEvents || snapshot.StdoutTruncated || snapshot.StderrTruncated {
			c.stopped = true
		}
		c.bytesSinceSnapshot = 0
		c.lastSnapshot = now
		shouldObserve = true
	}
	c.mu.Unlock()

	if shouldObserve && c.observe != nil {
		c.observe(snapshot)
	}
	return written, nil
}

func (c *managedJobOutputCapture) shouldObserveLocked(now time.Time) bool {
	if c.observe == nil || c.stopped {
		return false
	}
	if !c.emitted {
		return true
	}
	if c.bytesSinceSnapshot >= managedJobOutputSnapshotMinBytes {
		return true
	}
	if !c.lastSnapshot.IsZero() && now.Sub(c.lastSnapshot) >= managedJobOutputSnapshotMinInterval {
		return true
	}
	return false
}

func (c *managedJobOutputCapture) snapshotLocked() JobProjection {
	stdout := c.stdout.Capture()
	stderr := c.stderr.Capture()
	snapshot := JobProjection{
		Status: "running",
	}
	applyJobOutputCapture(&snapshot, stdout, stderr)
	return snapshot
}

type managedJobOutputStreamWriter struct {
	capture *managedJobOutputCapture
	stream  string
}

func (w managedJobOutputStreamWriter) Write(data []byte) (int, error) {
	if w.capture == nil {
		return len(data), nil
	}
	return w.capture.write(w.stream, data)
}
