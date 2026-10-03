package surface

import (
	"errors"
	"fmt"
	"os"
)

type FailureReason string

const (
	ContextNotFound        FailureReason = "context_not_found"
	ProviderUnavailable    FailureReason = "provider_unavailable"
	BridgeUnreachable      FailureReason = "bridge_unreachable"
	AppNotRunning          FailureReason = "app_not_running"
	PermissionDenied       FailureReason = "permission_denied"
	InvalidTarget          FailureReason = "invalid_target"
	InvalidCWD             FailureReason = "invalid_cwd"
	UnsupportedSurface     FailureReason = "unsupported_surface"
	MultiplexerUnreachable FailureReason = "multiplexer_unreachable"
	ServerIdentityMismatch FailureReason = "server_identity_mismatch"
	PaneNotFound           FailureReason = "pane_not_found"
	InstanceMismatch       FailureReason = "instance_mismatch"
	Unknown                FailureReason = "unknown"
)

type Failure struct {
	Reason FailureReason
	Err    error
}

func (e *Failure) Error() string                 { return fmt.Sprintf("%s: %v", e.Reason, e.Err) }
func (e *Failure) Unwrap() error                 { return e.Err }
func Fail(reason FailureReason, err error) error { return &Failure{reason, err} }
func Reason(err error) FailureReason {
	if err == nil {
		return ""
	}
	var failure *Failure
	if errors.As(err, &failure) {
		return failure.Reason
	}
	if errors.Is(err, os.ErrPermission) {
		return PermissionDenied
	}
	return Unknown
}

// Probe must not select tabs, activate apps or create sessions.
type ProbeableProvider interface{ Probe(ReturnTarget) error }
