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
		return errors.New("invalid Tabby window identifier")
	}
	conn, err := net.DialTimeout("unix", filepath.Join(p.directory(), window+".sock"), time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if err = json.NewEncoder(conn).Encode(request); err != nil {
		return err
	}
	return json.NewDecoder(io.LimitReader(conn, 8192)).Decode(response)
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
	ids := strings.Split(t.ContextID, ":")
	if len(ids) != 2 || !identifier.MatchString(ids[1]) {
		return t, nil
	}
	contexts, err := p.List(ids[0])
	if err != nil {
		return t, nil
	}
	for _, context := range contexts {
		if context.ContextID == t.ContextID {
			t.Capability = surface.ReturnExactContext
			break
		}
	}
	return t, nil
}
func (p Provider) Return(t surface.ReturnTarget) error {
	ids := strings.Split(t.ContextID, ":")
	if len(ids) != 2 || !identifier.MatchString(ids[1]) {
		return errors.New("invalid Tabby context")
	}
	var response struct {
		OK bool `json:"ok"`
	}
	if err := p.request(ids[0], map[string]string{"operation": "focus", "context": t.ContextID}, &response); err != nil {
		return err
	}
	if !response.OK {
		return errors.New("Tabby context expired")
	}
	return nil
}

type Context struct{ ContextID, Title string }

func (p Provider) List(window string) ([]Context, error) {
	var response struct {
		Contexts []Context `json:"contexts"`
	}
	err := p.request(window, map[string]string{"operation": "list"}, &response)
	return response.Contexts, err
}
