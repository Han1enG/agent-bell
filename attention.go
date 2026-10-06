package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/han1eng/agent-bell/internal/attention"
	"github.com/han1eng/agent-bell/internal/config"
	"github.com/han1eng/agent-bell/internal/debounce"
	"github.com/han1eng/agent-bell/internal/event"
	"github.com/han1eng/agent-bell/internal/notify"
	"github.com/han1eng/agent-bell/internal/surface"
	"github.com/han1eng/agent-bell/internal/surface/builtin"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

func statusCommand(args []string, out io.Writer) error {
	if len(args) > 1 || len(args) == 1 && args[0] != "--json" {
		return errors.New("usage: agentbell status [--json]")
	}
	home, _ := os.UserHomeDir()
	v, err := attentionStatus(home)
	if err != nil {
		return err
	}
	if len(args) == 1 {
		return json.NewEncoder(out).Encode(v)
	}
	for _, section := range []struct {
		name     string
		sessions []attention.Session
	}{{"NEEDS YOU", v.NeedsYou}, {"WORKING", v.Working}, {"READY", v.Recent}, {"RECENTLY CLOSED", v.Closed}} {
		fmt.Fprintln(out, section.name)
		for _, s := range section.sessions {
			label := string(s.Status)
			switch s.Attention {
			case attention.AttentionInput:
				label = "Waiting for input"
			case attention.AttentionApproval:
				label = "Approval required"
			case attention.AttentionError:
				label = "Failed"
			}
			at := s.StartedAt
			if s.AttentionAt != nil {
				at = *s.AttentionAt
			} else if s.FinishedAt != nil {
				at = *s.FinishedAt
			}
			agent := "Codex"
			if s.Agent == "claude" {
				agent = "Claude"
			}
			fmt.Fprintf(out, "\n%s\n%s · %s · %s\n", s.Project, agent, label, time.Since(at).Round(time.Second))
			if s.Summary != "" {
				fmt.Fprintln(out, s.Summary)
			}
		}
		fmt.Fprintln(out)
	}
	return nil
}
func attentionStatus(home string) (attention.Snapshot, error) {
	r, err := attention.RequestTo(attention.SocketPath(home), attention.Request{Version: 1, Command: "status"}, 200*time.Millisecond)
	if err == nil && r.State != nil {
		return *r.State, nil
	}
	if errors.Is(err, attention.ErrIncompatible) {
		return attention.Snapshot{}, err
	}
	cfg, cfgErr := config.Load(config.Path(home))
	if cfgErr != nil {
		cfg = config.Defaults()
	}
	m := attention.New(cfg.AttentionCenter.RetentionDays)
	store, e := attention.OpenStore(attention.DBPath(home), true)
	if e != nil {
		if _, e2 := os.Stat(attention.DBPath(home)); os.IsNotExist(e2) {
			return m.Snapshot(), nil
		}
		return attention.Snapshot{}, e
	}
	defer store.Close()
	if e = store.Load(m); e != nil {
		return attention.Snapshot{}, e
	}
	m.Reconcile(time.Now(), attention.ProcessProbe)
	m.Cleanup(time.Now())
	return m.Snapshot(), nil
}
func attentionControl(args []string) error {
	relocate := len(args) == 3 && args[0] == "relocate_session" && len(args[1]) <= 1024
	valid := len(args) == 1 && (args[0] == "pause" || args[0] == "resume" || args[0] == "clear_recent" || args[0] == "clear_all")
	single := len(args) == 2 && (args[0] == "remove_recent" || args[0] == "dismiss_session") && len(args[1]) > 0 && len(args[1]) <= 1024
	if !valid && !single && !relocate {
		return errors.New("usage: agentbell attention-control pause|resume|clear_recent|remove_recent SESSION_ID")
	}
	request := attention.Request{Version: 1, Command: args[0]}
	if single || relocate {
		request.SessionID = args[1]
	}
	if relocate {
		request.CWD = args[2]
	}
	home, _ := os.UserHomeDir()
	_, err := attention.RequestTo(attention.SocketPath(home), request, time.Second)
	return err
}
func residentNotificationAllowed(cfg config.Config, e event.AgentEvent) bool {
	return cfg.Allows(string(e.Type)) && (e.Type != event.Done || cfg.AttentionCenter.DoneNotifications)
}

// The embedded core is owned by AgentBell.app. EOF terminates it even after an
// app crash. There is no separately installed daemon or service.
func attentionHost(stdin io.Reader, out io.Writer) error {
	home, _ := os.UserHomeDir()
	cfg, err := config.Load(config.Path(home))
	if err != nil {
		cfg = config.Defaults()
	}
	if !cfg.AttentionCenter.Enabled {
		return errors.New("Attention Center disabled in config")
	}
	listener, cleanup, err := attention.Listen(home)
	if err != nil {
		return err
	}
	defer cleanup()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer cancel()
	go func() { _, _ = io.Copy(io.Discard, stdin); cancel() }()
	m := attention.New(cfg.AttentionCenter.RetentionDays)
	store, storeErr := attention.OpenStore(attention.DBPath(home), false)
	storageError := ""
	if storeErr == nil {
		if storeErr = store.Load(m); storeErr != nil {
			store.Close()
			store = nil
		}
	} else {
		store = nil
	}
	if storeErr != nil {
		storageError = storeErr.Error()
		writeDebugLog("storage_error=%v", storeErr)
	}
	if store != nil {
		defer store.Close()
	}
	for id, s := range m.Sessions {
		if title := attention.LookupTitle(home, s.Agent, strings.TrimPrefix(id, s.Agent+":"), s.CWD); title != "" {
			s.Title = title
			m.Sessions[id] = s
		}
	}
	m.Reconcile(time.Now(), attention.ProcessProbe)
	m.Cleanup(time.Now())
	var mu sync.Mutex
	var storeMu sync.Mutex
	var pending []attention.Transition
	dirty := true
	observed := map[string]bool{}
	observedAgents := func() []string {
		agents := []string{}
		for _, name := range []string{"claude", "codex"} {
			if observed[name] {
				agents = append(agents, name)
			}
		}
		return agents
	}
	updates := make(chan attention.Snapshot, 1)
	notifications := make(chan event.AgentEvent, 64)
	nativeNotifications := make(chan notify.Content, 64)
	emit := func() {
		v := m.Snapshot()
		v.RecentLimit = cfg.AttentionCenter.RecentLimit
		v.AppVersion = version
		v.StorageError = storageError
		v.ObservedAgents = observedAgents()
		select {
		case <-updates:
		default:
		}
		updates <- v
	}
	mu.Lock()
	emit()
	mu.Unlock()
	go func() {
		encoder := json.NewEncoder(out)
		for {
			select {
			case v := <-updates:
				if encoder.Encode(v) != nil {
					cancel()
					return
				}
			case content := <-nativeNotifications:
				if encoder.Encode(content) != nil {
					cancel()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		for {
			select {
			case e := <-notifications:
				// Configuration and debounce apply only after the state has been accepted.
				current, e2 := config.Load(config.Path(home))
				if e2 != nil {
					current = config.Defaults()
				}
				mu.Lock()
				paused := m.Paused
				mu.Unlock()
				if !paused && !attention.UserDisabled(home) && residentNotificationAllowed(current, e) && !debounce.Suppressed(home, e, time.Now()) {
					select {
					case nativeNotifications <- notify.ContentFor(e):
					case <-ctx.Done():
						return
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	flush := func() {
		// Always take the store lock before the engine lock, including control IPC.
		// This prevents stale flushes overwriting a committed dismissal, without
		// blocking ordinary hooks on SQLite serialization and disk I/O.
		storeMu.Lock()
		defer storeMu.Unlock()
		mu.Lock()
		if !dirty {
			mu.Unlock()
			return
		}
		copyEngine := attention.New(m.RetentionDays)
		copyEngine.Paused = m.Paused
		for id, s := range m.Sessions {
			copyEngine.Sessions[id] = s
		}
		batch := pending
		pending = nil
		dirty = false
		mu.Unlock()
		if store != nil {
			if err := store.Save(copyEngine, batch); err != nil {
				mu.Lock()
				storageError = err.Error()
				dirty = true
				pending = append(batch, pending...)
				emit()
				mu.Unlock()
				writeDebugLog("storage_error=%v", err)
			}
		}
	}

	done := make(chan error, 1)
	go func() {
		done <- attention.Serve(ctx, listener, func(r attention.Request) attention.Response {
			// Recovery checks a copied snapshot; CLI help never blocks hook ingestion.
			if r.Command == "recovery" {
				mu.Lock()
				s, ok := m.Sessions[r.SessionID]
				for id, other := range m.Sessions {
					if id != s.ID && s.NativeSessionID != "" && s.NativeSessionID == other.NativeSessionID && s.Agent == other.Agent && other.RuntimeState != attention.RuntimeExited {
						s.ConcurrentRuntime = true
					}
				}
				mu.Unlock()
				if !ok {
					return attention.Response{Error: "session unavailable"}
				}
				result := attention.Recovery(s)
				return attention.Response{Recovery: &result}
			}
			persistControl := r.Command == "dismiss_session" || r.Command == "clear_all" || r.Command == "remove_recent" || r.Command == "clear_recent" || r.Command == "relocate_session"
			if persistControl {
				storeMu.Lock()
				defer storeMu.Unlock()
			}
			mu.Lock()
			defer mu.Unlock()
			v := attention.Response{Version: 1}
			switch r.Command {
			case "", "event":
				if r.Event == nil {
					v.Error = "missing event"
					break
				}
				e := *r.Event
				e.Normalize()
				// Never admit raw payloads to state; the hook already bounded the summary.
				e.Raw = nil
				e.Message = notify.Summary(e.Message)
				changed, err := m.Apply(e)
				if err != nil {
					v.Error = err.Error()
					break
				}
				if changed {
					id, _ := attention.SessionKey(e)
					pending = append(pending, attention.Transition{ID: id, Type: string(e.Type), Timestamp: e.Timestamp.Format(time.RFC3339Nano)})
				}
				id, keyErr := attention.SessionKey(e)
				if keyErr == nil {
					if _, ok := m.Sessions[id]; ok {
						observed[e.Source] = true
					}
				}
				dirty = true
				emit()
				accepted := false
				for _, s := range m.Sessions {
					if s.Agent == e.Source && s.NativeSessionID == e.SessionID && s.DismissedAt == nil && s.RuntimeState != attention.RuntimeExited && s.LastEventType == string(e.Type) && s.UpdatedAt.Equal(e.Timestamp) {
						accepted = true
					}
				}
				if !m.Paused && (accepted || e.Type == event.EventPermissionRequest) && (e.Type == event.Done || e.Type == event.NeedsInput || e.Type == event.NeedsApproval || e.Type == event.Error || e.Type == event.EventPermissionRequest) {
					select {
					case notifications <- e:
					default:
						writeDebugLog("notification_queue_full=true")
					}
				}
			case "status":
				s := m.Snapshot()
				s.AppVersion = version
				s.StorageError = storageError
				s.ObservedAgents = observedAgents()
				v.State = &s
			case "pause":
				m.Paused = true
				dirty = true
				emit()
			case "resume":
				m.Paused = false
				dirty = true
				emit()
			case "dismiss_session", "remove_recent":
				if r.SessionID == "" || len(r.SessionID) > 1024 {
					v.Error = "missing or invalid session ID"
					break
				}
				remove := m.RemoveRecent
				if r.Command == "dismiss_session" {
					remove = func(id string) error { return m.Dismiss(id, time.Now()) }
				}
				if err := remove(r.SessionID); err != nil {
					v.Error = err.Error()
					break
				}
				dirty = true
				emit()
			case "clear_all":
				m.ClearAll(time.Now())
				dirty = true
				emit()
			case "relocate_session":
				s, ok := m.Sessions[r.SessionID]
				if !ok || s.RuntimeState != attention.RuntimeExited {
					v.Error = "only closed sessions can choose a recovery directory"
					break
				}
				if !filepath.IsAbs(r.CWD) || len(r.CWD) > 4096 {
					v.Error = "invalid project directory"
					break
				}
				info, err := os.Stat(r.CWD)
				if err != nil || !info.IsDir() {
					v.Error = "project directory unavailable"
					break
				}
				s.CWD = filepath.Clean(r.CWD)
				m.Sessions[s.ID] = s
				dirty = true
				emit()
			case "invalidate_context":
				s, ok := m.Sessions[r.SessionID]
				if !ok {
					v.Error = "session unavailable"
					break
				}
				if attention.ProcessProbe(s) == attention.Exited {
					m.Reconcile(time.Now(), func(other attention.Session) attention.ProbeResult {
						if other.ID == s.ID {
							return attention.Exited
						}
						return attention.Unknown
					})
				}
				s = m.Sessions[r.SessionID]
				s.SurfaceState = "unavailable"
				m.Sessions[r.SessionID] = s
				dirty = true
				emit()
			case "get_session", "recovery", "resume_session":
				s, ok := m.Sessions[r.SessionID]
				if !ok {
					v.Error = "session unavailable"
					break
				}
				for id, other := range m.Sessions {
					if id != s.ID && s.NativeSessionID != "" && other.NativeSessionID == s.NativeSessionID && other.Agent == s.Agent && other.RuntimeState != attention.RuntimeExited {
						s.ConcurrentRuntime = true
					}
				}
				v.Session = &s
				v.Recovery = &attention.ResumeResult{Reason: "automatic resume is unavailable; use Copy Resume Command"}
				if r.Command == "resume_session" {
					v.Error = "automatic resume is unavailable; use Copy Resume Command"
				}
			case "clear_recent":
				m.ClearRecent()
				dirty = true
				emit()
			default:
				v.Error = "unknown IPC command"
			}
			if persistControl {
				if store == nil {
					v.Error = "session hidden for this run; persistence unavailable"
				} else if err := store.Save(m, pending); err != nil {
					v.Error = err.Error()
					storageError = err.Error()
				} else {
					pending = nil
					dirty = false
				}
			}
			return v
		})
	}()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	reconcile := time.NewTicker(5 * time.Second)
	defer reconcile.Stop()
	for {
		select {
		case <-ticker.C:
			flush()
		case <-reconcile.C:
			mu.Lock()
			sessions := []attention.Session{}
			for _, s := range m.Sessions {
				if s.RuntimeState != attention.RuntimeExited && s.DismissedAt == nil {
					sessions = append(sessions, s)
				}
			}
			mu.Unlock()
			probes := map[string]attention.ProbeResult{}
			origins := map[string]attention.Session{}
			surfaces := map[string]string{}
			for _, s := range sessions {
				origins[s.ID] = s
				surfaces[s.ID] = probeSurface(s)
				probes[s.ID] = attention.ProcessProbe(s)
			}
			mu.Lock()
			m.Reconcile(time.Now(), func(s attention.Session) attention.ProbeResult {
				old, ok := origins[s.ID]
				if !ok || s.RuntimeInstanceID != old.RuntimeInstanceID || s.ProcessID != old.ProcessID || s.ProcessIdentity != old.ProcessIdentity {
					return attention.Unknown
				}
				return probes[s.ID]
			})
			for id, state := range surfaces {
				s := m.Sessions[id]
				if s.RuntimeInstanceID == origins[id].RuntimeInstanceID {
					s.SurfaceState = state
					m.Sessions[id] = s
				}
			}
			m.Cleanup(time.Now())
			dirty = true
			emit()
			mu.Unlock()
			flush()
		case err := <-done:
			cancel()
			flush()
			return err
		case <-ctx.Done():
			<-done
			flush()
			return nil
		}
	}
}
func printAttentionDoctor(home string, out io.Writer) {
	fmt.Fprintln(out, "\nAttention Center")
	exe, _ := os.Executable()
	printAppInstallation(home, exe, out)
	r, err := attention.RequestTo(attention.SocketPath(home), attention.Request{Version: 1, Command: "status"}, 200*time.Millisecond)
	if err != nil {
		fmt.Fprintln(out, "○ App not running / IPC unavailable (notification fallback remains available)")
	} else if r.State != nil {
		fmt.Fprintf(out, "✓ IPC protocol 1 · App %s\n✓ %d active sessions · %d need attention\n", r.State.AppVersion, len(r.State.Working)+len(r.State.NeedsYou), len(r.State.NeedsYou))
		if r.State.AppVersion != version {
			fmt.Fprintln(out, "○ CLI/App version mismatch")
		}
		if r.State.StorageError != "" {
			fmt.Fprintf(out, "○ Storage: %s\n", r.State.StorageError)
		}
	}
	store, err := attention.OpenStore(attention.DBPath(home), true)
	if err != nil {
		fmt.Fprintf(out, "○ Session Store: %v\n", err)
		return
	}
	defer store.Close()
	if err = store.Health(); err != nil {
		fmt.Fprintf(out, "○ Session Store: %v\n", err)
	} else {
		fmt.Fprintln(out, "✓ SQLite schema v1 · database healthy")
	}
}

// Surface availability cannot establish process exit.
func sessionAlive(s attention.Session) bool { return attention.ProcessProbe(s) != attention.Exited }

func probeSurface(s attention.Session) string {
	if s.ReturnTarget == nil {
		return "unknown"
	}
	t := *s.ReturnTarget
	if t.Surface != "tabby" && t.Surface != "jetbrains" && t.Surface != "tmux" {
		return "unknown"
	}
	p := builtin.Registry("").Provider(t.Surface)
	if probe, ok := p.(surface.ProbeableProvider); ok {
		err := probe.Probe(t)
		if err == nil {
			return "available"
		}
		switch surface.Reason(err) {
		case surface.ContextNotFound, surface.PaneNotFound, surface.InstanceMismatch, surface.ServerIdentityMismatch:
			return "unavailable"
		}
	}
	return "unknown"
}
