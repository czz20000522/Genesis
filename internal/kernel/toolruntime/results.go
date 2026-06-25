package toolruntime

type RequestInvalidProjection struct {
	Status   string       `json:"status"`
	Tool     string       `json:"tool"`
	Executed bool         `json:"executed"`
	Error    RequestError `json:"error"`
}

type RequestError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type CapabilityProjection struct {
	Name            string `json:"name"`
	SideEffectLevel string `json:"side_effect_level"`
	ExecutionKind   string `json:"execution_kind"`
	Status          string `json:"status"`
}

type OperationResult struct {
	Status              string `json:"status"`
	Executed            bool   `json:"executed"`
	ExitCode            *int   `json:"exit_code,omitempty"`
	TimedOut            bool   `json:"timed_out,omitempty"`
	TimeoutReason       string `json:"timeout_reason,omitempty"`
	Interrupted         bool   `json:"interrupted,omitempty"`
	InterruptReason     string `json:"interrupt_reason,omitempty"`
	ElapsedMs           int64  `json:"elapsed_ms,omitempty"`
	Stdout              string `json:"stdout,omitempty"`
	Stderr              string `json:"stderr,omitempty"`
	StdoutTruncated     bool   `json:"stdout_truncated,omitempty"`
	StderrTruncated     bool   `json:"stderr_truncated,omitempty"`
	StdoutOriginalBytes int    `json:"stdout_original_bytes,omitempty"`
	StderrOriginalBytes int    `json:"stderr_original_bytes,omitempty"`
	StdoutOmittedBytes  int    `json:"stdout_omitted_bytes,omitempty"`
	StderrOmittedBytes  int    `json:"stderr_omitted_bytes,omitempty"`
	OutputTruncation    string `json:"output_truncation,omitempty"`
}

type ManagedJobResult struct {
	Status        string `json:"status"`
	Executed      bool   `json:"executed"`
	JobID         string `json:"job_id"`
	VisibleOutput string `json:"visible_output"`
}

type JobControlResult struct {
	Status          string `json:"status"`
	Executed        bool   `json:"executed"`
	JobID           string `json:"job_id"`
	Tool            string `json:"tool,omitempty"`
	CancelRequested bool   `json:"cancel_requested,omitempty"`
	VisibleOutput   string `json:"visible_output,omitempty"`
	ExitCode        *int   `json:"exit_code,omitempty"`
	Stdout          string `json:"stdout,omitempty"`
	Stderr          string `json:"stderr,omitempty"`
	StdoutTruncated bool   `json:"stdout_truncated,omitempty"`
	StderrTruncated bool   `json:"stderr_truncated,omitempty"`
}
