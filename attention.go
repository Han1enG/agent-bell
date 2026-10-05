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
	}{{"NEEDS YOU", v.NeedsYou}, {"WORKING", v.Working}, {"RECENT", v.Recent}} {
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
	m.Reconcile(time.Now(), sessionAlive)
	m.Cleanup(time.Now())
	return m.Snapshot(), nil
}
func attentionControl(args []string) error {
	valid := len(args) == 1 && (args[0] == "pause" || args[0] == "resume" || args[0] == "clear_recent")
	single := len(args) == 2 && args[0] == "remove_recent" && len(args[1]) > 0 && len(args[1]) <= 1024
	if !valid && !single {
		return errors.New("usage: agentbell attention-control pause|resume|clear_recent|remove_recent SESSION_ID")
	}
	request := attention.Request{Version: 1, Command: args[0]}
	if single {
		request.SessionID = args[1]
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
	m.Reconcile(time.Now(), sessionAlive)
	m.Cleanup(time.Now())
	var mu sync.Mutex
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
				if !paused && residentNotificationAllowed(current, e) && !debounce.Suppressed(home, e, time.Now()) {
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
		mu.Lock()
		copyEngine := attention.New(m.RetentionDays)
		copyEngine.Paused = m.Paused
		for id, s := range m.Sessions {
			copyEngine.Sessions[id] = s
		}
		batch := pending
		pending = nil
		if !dirty {
			mu.Unlock()
			return
		}
		dirty = false
		mu.Unlock()
		if store != nil {
			if err := store.Save(copyEngine, batch); err != nil {
				mu.Lock()
				storageError = err.Error()
				emit()
				mu.Unlock()
				writeDebugLog("storage_error=%v", err)
			}
		}
	}
	done := make(chan error, 1)
	go func() {
		done <- attention.Serve(ctx, listener, func(r attention.Request) attention.Response {
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
				if !m.Paused && (e.Type == event.Done || e.Type == event.NeedsInput || e.Type == event.NeedsApproval || e.Type == event.Error || e.Type == event.EventPermissionRequest) {
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
			case "remove_recent":
				if r.SessionID == "" || len(r.SessionID) > 1024 {
					v.Error = "missing or invalid session ID"
					break
				}
				if err := m.RemoveRecent(r.SessionID); err != nil {
					v.Error = err.Error()
					break
				}
				dirty = true
				emit()
			case "clear_recent":
				m.ClearRecent()
				dirty = true
				emit()
			default:
				v.Error = "unknown IPC command"
			}
			return v
		})
	}()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	reconcile := time.NewTicker(time.Minute)
	defer reconcile.Stop()
	for {
		select {
		case <-ticker.C:
			flush()
		case <-reconcile.C:
			mu.Lock()
			sessions := m.Snapshot()
			mu.Unlock()
			dead := map[string]bool{}
			for _, s := range append(sessions.Working, sessions.NeedsYou...) {
				dead[s.ID] = !sessionAlive(s)
			}
			mu.Lock()
			m.Reconcile(time.Now(), func(s attention.Session) bool { return !dead[s.ID] })
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

// Only probes local providers that inspect lifecycle metadata without Apple
// events or focusing a surface. Unreachable bridges are inconclusive.
func sessionAlive(s attention.Session) bool {
	if !attention.ProcessAlive(s) {
		return false
	}
	if s.ReturnTarget != nil && s.ReturnTarget.ContextID != "" {
		t := *s.ReturnTarget
		if t.Surface == "tabby" || t.Surface == "jetbrains" || t.Surface == "tmux" {
			if p, ok := builtin.Registry("").Provider(t.Surface).(surface.ProbeableProvider); ok {
				if err := p.Probe(t); surface.Reason(err) == surface.ContextNotFound {
					writeDebugLog("session_stale=true session=%s", s.ID)
					return false
				}
			}
		}
	}
	return true
}
