// Package jetbrains talks to the private AgentBell IDE plugin bridge.
package jetbrains

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/han1eng/agent-bell/internal/surface"
)

var identifier = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)

type Provider struct {
	Directory string
	Run       surface.Runner
}
type Context struct {
	ContextID, Title string
	Selected         bool
}

func (p Provider) directory() string {
	if p.Directory != "" {
		return p.Directory
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "agentbell", "jetbrains")
}
func (p Provider) Name() string { return "jetbrains" }
func (p Provider) CanHandle(t surface.ReturnTarget) bool {
	return t.Surface == "jetbrains" && t.Capability == surface.ReturnExactContext
}
func splitContext(id string) ([]string, error) {
	parts := strings.Split(id, ":")
	if len(parts) != 2 || !identifier.MatchString(parts[0]) || !identifier.MatchString(parts[1]) {
		return nil, surface.Fail(surface.InvalidTarget, errors.New("invalid JetBrains context"))
	}
	return parts, nil
}
func (p Provider) request(instance string, request any, response any) error {
	if !identifier.MatchString(instance) {
		return surface.Fail(surface.InvalidTarget, errors.New("invalid JetBrains instance"))
	}
	conn, err := net.DialTimeout("unix", filepath.Join(p.directory(), instance+".sock"), time.Second)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return surface.Fail(surface.PermissionDenied, err)
		}
		return surface.Fail(surface.BridgeUnreachable, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if err = json.NewEncoder(conn).Encode(request); err != nil {
		return surface.Fail(surface.BridgeUnreachable, err)
	}
	if err := json.NewDecoder(io.LimitReader(conn, 65536)).Decode(response); err != nil {
		return surface.Fail(surface.BridgeUnreachable, err)
	}
	return nil
}
func (p Provider) List(instance string) ([]Context, error) {
	var response struct {
		OK       bool      `json:"ok"`
		Contexts []Context `json:"contexts"`
	}
	err := p.request(instance, map[string]string{"operation": "list"}, &response)
	if err == nil && !response.OK {
		err = surface.Fail(surface.ProviderUnavailable, errors.New("JetBrains bridge unavailable"))
	}
	return response.Contexts, err
}
func (p Provider) Detect(c surface.DetectContext) (*surface.ReturnTarget, error) {
	if c.Env == nil {
		c.Env = os.Getenv
	}
	if c.Env("AGENTBELL_SURFACE") != "jetbrains" {
		return nil, nil
	}
	target, err := (surface.GenericProvider{Run: p.Run}).Detect(c)
	if err != nil {
		return nil, err
	}
	if p.Probe(*target) == nil {
		target.Capability = surface.ReturnExactContext
	}
	return target, nil
}
func (p Provider) Return(target surface.ReturnTarget) error {
	if !p.CanHandle(target) {
		return surface.Fail(surface.UnsupportedSurface, errors.New("unsupported JetBrains target"))
	}
	parts, err := splitContext(target.ContextID)
	if err != nil {
		return err
	}
	var response struct {
		OK     bool   `json:"ok"`
		Reason string `json:"reason"`
	}
	if err = p.request(parts[0], map[string]string{"operation": "focus", "context": target.ContextID}, &response); err != nil {
		return err
	}
	if !response.OK {
		if response.Reason == string(surface.ProviderUnavailable) {
			return surface.Fail(surface.ProviderUnavailable, errors.New("JetBrains UI unavailable"))
		}
		return surface.Fail(surface.ContextNotFound, errors.New("JetBrains context expired or unavailable"))
	}
	return nil
}

func (p Provider) Probe(t surface.ReturnTarget) error {
	parts, err := splitContext(t.ContextID)
	if err != nil {
		return err
	}
	contexts, err := p.List(parts[0])
	if err != nil {
		return err
	}
	for _, context := range contexts {
		if context.ContextID == t.ContextID {
			return nil
		}
	}
	return surface.Fail(surface.ContextNotFound, errors.New("JetBrains context expired"))
}
