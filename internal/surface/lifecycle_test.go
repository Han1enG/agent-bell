package surface

import (
	"errors"
	"testing"
)

type expiredProbe struct {
	failingProvider
	returns *int
}

func (p expiredProbe) Probe(ReturnTarget) error {
	return Fail(ContextNotFound, errors.New("closed after notification"))
}
func (p expiredProbe) CanHandle(t ReturnTarget) bool { return t.Capability == ReturnExactContext }
func (p expiredProbe) Return(ReturnTarget) error     { *p.returns++; return nil }
func TestClickRevalidatesEncodedTarget(t *testing.T) {
	returns := 0
	var attempts []ReturnCapability
	exact := expiredProbe{returns: &returns}
	var reasons []FailureReason
	err := (Manager{Providers: []SurfaceProvider{exact, failingProvider{&attempts, ReturnApp}}, OnAttempt: func(_ string, cap ReturnCapability, err error) {
		if cap == ReturnExactContext {
			reasons = append(reasons, Reason(err))
		}
	}}).ReturnToContext(ReturnTarget{ContextID: "was-live", AppBundleID: "app", Capability: ReturnExactContext})
	if err != nil || returns != 0 || len(reasons) == 0 || reasons[0] != ContextNotFound {
		t.Fatal(err, returns, reasons)
	}
}
func TestFailureReasons(t *testing.T) {
	for _, reason := range []FailureReason{ContextNotFound, ProviderUnavailable, BridgeUnreachable, AppNotRunning, PermissionDenied, InvalidTarget, InvalidCWD, UnsupportedSurface, Unknown} {
		if Reason(errors.Join(Fail(reason, errors.New("detail")))) != reason {
			t.Fatal(reason)
		}
	}
	if Reason(nil) != "" || Reason(errors.New("untyped")) != Unknown {
		t.Fatal("unexpected default")
	}
}
