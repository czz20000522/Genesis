package kernel

import (
	"context"
	"fmt"
	"strings"
	"sync"
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

		stdout, stderr, exitCode, err := runShellProcessContext(jobCtx, job.CWD, job.Command)
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
