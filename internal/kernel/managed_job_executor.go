package kernel

import (
	"context"
	"fmt"
	"io"
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

type ManagedJobStartRequest struct {
	Job      JobProjection
	Observe  func(JobProjection)
	Complete func(JobProjection)
}

type localManagedJobExecutor struct {
	root   context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	running map[string]*localManagedJob
}

type localManagedJob struct {
	cancel context.CancelFunc
	reason string
}

func newLocalManagedJobExecutor() *localManagedJobExecutor {
	root, cancel := context.WithCancel(context.Background())
	return &localManagedJobExecutor{
		root:    root,
		cancel:  cancel,
		running: map[string]*localManagedJob{},
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
	jobCtx, cancel := context.WithCancel(e.root)
	active := &localManagedJob{cancel: cancel}
	e.mu.Lock()
	if _, exists := e.running[jobID]; exists {
		e.mu.Unlock()
		cancel()
		return fmt.Errorf("job %s is already running", jobID)
	}
	e.running[jobID] = active
	e.mu.Unlock()

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer func() {
			e.mu.Lock()
			delete(e.running, jobID)
			e.mu.Unlock()
		}()

		output := newManagedJobOutputCapture(request.Observe)
		exitCode, err := runShellProcessContextWithOutput(jobCtx, job.CWD, job.Command, output.stdoutWriter(), output.stderrWriter())
		stdout, stderr := output.capture()
		finished := job
		if err == nil {
			finished.ExitCode = &exitCode
		}
		applyJobOutputCapture(&finished, stdout, stderr)

		switch {
		case jobCtx.Err() != nil:
			finished.Status = "cancelled"
			finished.CancelReason = e.cancelReason(jobID)
		case err != nil:
			finished.Status = "failed"
			finished.FailureReason = "shell_runtime_failed: " + err.Error()
		case exitCode == 0:
			finished.Status = "completed"
		default:
			finished.Status = "failed"
		}
		request.Complete(finished)
	}()
	return nil
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
	active.reason = strings.TrimSpace(reason)
	cancel := active.cancel
	e.mu.Unlock()
	cancel()
	return true, nil
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
