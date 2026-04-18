package capsule

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// RuntimeKind enumerates the supported execution environments for a Capsule.
type RuntimeKind string

const (
	// RuntimeInProcess runs the capsule directly inside the host process.
	// This is the default for built-in capsules and lightweight extensions.
	RuntimeInProcess RuntimeKind = "inprocess"

	// RuntimeSidecar runs the capsule as an external subprocess managed by
	// the host. Suited for heavy dependencies (e.g. digital-human live,
	// ffmpeg-bound workloads) that must not bloat the host binary.
	RuntimeSidecar RuntimeKind = "sidecar"

	// RuntimeContainer runs the capsule inside a container image, via the
	// host's existing sandbox plumbing (internal/execution/sandbox).
	// Reserved for future implementation.
	RuntimeContainer RuntimeKind = "container"
)

// RuntimeStatus is a snapshot of a Runtime's health / liveness.
type RuntimeStatus struct {
	Kind     RuntimeKind `json:"kind"`
	Running  bool        `json:"running"`
	Started  time.Time   `json:"started,omitempty"`
	PID      int         `json:"pid,omitempty"`
	Message  string      `json:"message,omitempty"`
	MemoryMB int         `json:"memory_mb,omitempty"`
	CPUPct   int         `json:"cpu_pct,omitempty"`
}

// Runtime is the interface every Capsule implementation provides to describe
// and drive its own execution environment.
//
// Kind tells the registry which flavor of runtime this is (useful for
// filtering / UI). Start / Stop are idempotent: multiple calls in the same
// state MUST be no-ops and return nil.
type Runtime interface {
	// Kind returns the runtime flavor.
	Kind() RuntimeKind

	// Start brings the runtime online. For InProcess this typically just
	// sets a flag and returns; for Sidecar it launches the subprocess and
	// waits for the health check.
	Start(ctx context.Context, env *Env) error

	// Stop shuts the runtime down cleanly. Implementations MUST tolerate
	// being called when the runtime is not running.
	Stop(ctx context.Context) error

	// Status returns a snapshot of the runtime's health.
	Status() RuntimeStatus
}

// ── InProcessRuntime ────────────────────────────────────────────────────────

// InProcessRuntime is the zero-overhead runtime used by built-in and script
// capsules. It only tracks started/stopped state and timestamps — the capsule
// body runs in the host process.
//
// An optional OnStart / OnStop callback lets capsule authors wire lifecycle
// work (e.g. spinning up background goroutines) without implementing the
// full Runtime interface.
type InProcessRuntime struct {
	OnStart func(ctx context.Context, env *Env) error
	OnStop  func(ctx context.Context) error

	running int32 // atomic bool (0/1)
	started atomic.Value // time.Time
	mu      sync.Mutex   // guards OnStart/OnStop idempotency
}

// NewInProcessRuntime creates a no-callback in-process runtime.
// This is suitable for capsules whose work is entirely triggered by
// skill invocations (no background activity).
func NewInProcessRuntime() *InProcessRuntime {
	return &InProcessRuntime{}
}

// Kind returns RuntimeInProcess.
func (r *InProcessRuntime) Kind() RuntimeKind { return RuntimeInProcess }

// Start marks the runtime running and invokes OnStart (if set).
// Idempotent: calling Start on an already-running runtime returns nil.
func (r *InProcessRuntime) Start(ctx context.Context, env *Env) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if atomic.LoadInt32(&r.running) == 1 {
		return nil
	}
	if r.OnStart != nil {
		if err := r.OnStart(ctx, env); err != nil {
			return fmt.Errorf("onstart: %w", err)
		}
	}
	atomic.StoreInt32(&r.running, 1)
	r.started.Store(time.Now())
	return nil
}

// Stop marks the runtime stopped and invokes OnStop (if set).
// Idempotent: calling Stop on a stopped runtime returns nil.
func (r *InProcessRuntime) Stop(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if atomic.LoadInt32(&r.running) == 0 {
		return nil
	}
	if r.OnStop != nil {
		if err := r.OnStop(ctx); err != nil {
			return fmt.Errorf("onstop: %w", err)
		}
	}
	atomic.StoreInt32(&r.running, 0)
	return nil
}

// Status reports current liveness.
func (r *InProcessRuntime) Status() RuntimeStatus {
	status := RuntimeStatus{
		Kind:    RuntimeInProcess,
		Running: atomic.LoadInt32(&r.running) == 1,
	}
	if v, ok := r.started.Load().(time.Time); ok {
		status.Started = v
	}
	return status
}

// ── SidecarRuntime (stub) ───────────────────────────────────────────────────

// SidecarRuntime is a placeholder for external-process capsules.
// Real implementation will wrap os/exec.Cmd with health-check polling,
// restart policy, log piping, and resource limits. Lives here so capsule
// manifests with Runtime.Kind == "sidecar" can declare intent today and
// the host can validate the manifest without the driver being complete.
type SidecarRuntime struct {
	Spec RuntimeSpec

	running atomic.Int32
	started atomic.Value // time.Time
	pid     atomic.Int32
}

// NewSidecarRuntime creates a Sidecar runtime driver backed by the given spec.
// Note: the driver itself is a stub — Start returns an error explaining the
// feature is not yet implemented. Once the driver lands, Start will fork the
// subprocess and wait for the health probe.
func NewSidecarRuntime(spec RuntimeSpec) *SidecarRuntime {
	return &SidecarRuntime{Spec: spec}
}

// Kind returns RuntimeSidecar.
func (r *SidecarRuntime) Kind() RuntimeKind { return RuntimeSidecar }

// Start returns ErrSidecarNotImplemented until the sidecar driver ships.
func (r *SidecarRuntime) Start(ctx context.Context, env *Env) error {
	return ErrSidecarNotImplemented
}

// Stop is a no-op (the process never started).
func (r *SidecarRuntime) Stop(ctx context.Context) error { return nil }

// Status reports a disabled sidecar.
func (r *SidecarRuntime) Status() RuntimeStatus {
	return RuntimeStatus{
		Kind:    RuntimeSidecar,
		Running: false,
		Message: "sidecar runtime not implemented yet",
	}
}

// ErrSidecarNotImplemented is returned until the sidecar driver lands.
var ErrSidecarNotImplemented = fmt.Errorf("capsule: sidecar runtime not implemented (scheduled for next milestone)")
