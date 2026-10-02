package tabby

import (
	"encoding/json"
	"errors"
	"github.com/han1eng/agent-bell/internal/surface"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var identifier = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)

type Provider struct{ Directory string }

func (p Provider) directory() string {
	if p.Directory != "" {
		return p.Directory
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "agentbell", "tabby")
}
func (p Provider) Name() string { return "tabby" }
func (p Provider) CanHandle(t surface.ReturnTarget) bool {
	return t.Surface == "tabby" && t.Capability == surface.ReturnExactContext
}
func (p Provider) request(window string, request any, response any) error {
	if !identifier.MatchString(window) {
		return surface.Fail(surface.InvalidTarget, errors.New("invalid Tabby window identifier"))
	}
	conn, err := net.DialTimeout("unix", filepath.Join(p.directory(), window+".sock"), time.Second)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return surface.Fail(surface.PermissionDenied, err)
		}
		return surface.Fail(surface.BridgeUnreachable, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if err = json.NewEncoder(conn).Encode(request); err != nil {
		return surface.Fail(surface.BridgeUnreachable, err)
	}
	if err := json.NewDecoder(io.LimitReader(conn, 65536)).Decode(response); err != nil {
		return surface.Fail(surface.BridgeUnreachable, err)
	}
	return nil
}
func (p Provider) Detect(c surface.DetectContext) (*surface.ReturnTarget, error) {
	if c.Env == nil {
		c.Env = os.Getenv
	}
	if c.Env("AGENTBELL_SURFACE") != "tabby" {
		return nil, nil
	}
	t, err := (surface.GenericProvider{}).Detect(c)
	if err != nil {
		return nil, err
	}
	if p.Probe(*t) == nil {
		t.Capability = surface.ReturnExactContext
	}
	return t, nil
}
func (p Provider) Return(t surface.ReturnTarget) error {
	ids := strings.Split(t.ContextID, ":")
	if len(ids) != 2 || !identifier.MatchString(ids[1]) {
		return surface.Fail(surface.InvalidTarget, errors.New("invalid Tabby context"))
	}
	var response struct {
		OK     bool   `json:"ok"`
		Reason string `json:"reason"`
	}
	if err := p.request(ids[0], map[string]string{"operation": "focus", "context": t.ContextID}, &response); err != nil {
		return err
	}
	if !response.OK {
		if response.Reason == string(surface.UnsupportedSurface) {
			return surface.Fail(surface.UnsupportedSurface, errors.New("Tabby pane focus unavailable"))
		}
		return surface.Fail(surface.ContextNotFound, errors.New("Tabby context expired"))
	}
	return nil
}

type Context struct {
	ContextID, Title string
	CanFocus         *bool
}

func (p Provider) List(window string) ([]Context, error) {
	var response struct {
		Contexts *[]Context `json:"contexts"`
	}
	err := p.request(window, map[string]string{"operation": "list"}, &response)
	if err != nil {
		return nil, err
	}
	if response.Contexts == nil {
		return nil, surface.Fail(surface.ProviderUnavailable, errors.New("invalid Tabby list response"))
	}
	return *response.Contexts, nil
}

func (p Provider) Probe(t surface.ReturnTarget) error {
	ids := strings.Split(t.ContextID, ":")
	if len(ids) != 2 || !identifier.MatchString(ids[0]) || !identifier.MatchString(ids[1]) {
		return surface.Fail(surface.InvalidTarget, errors.New("invalid Tabby context"))
	}
	contexts, err := p.List(ids[0])
	if err != nil {
		return err
	}
	for _, context := range contexts {
		if context.ContextID == t.ContextID {
			if context.CanFocus != nil && !*context.CanFocus {
				return surface.Fail(surface.UnsupportedSurface, errors.New("Tabby pane focus unavailable"))
			}
			return nil
		}
	}
	return surface.Fail(surface.ContextNotFound, errors.New("Tabby context expired"))
}
