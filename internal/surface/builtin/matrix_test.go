package builtin

import (
	"os"
	"strings"
	"testing"
)

func TestREADMEIsGeneratedFromDeclaredCapabilities(t *testing.T) {
	b, err := os.ReadFile("../../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	start := "<!-- agentbell-capabilities:start -->\n"
	end := "<!-- agentbell-capabilities:end -->"
	a := strings.Index(s, start)
	z := strings.Index(s, end)
	if a < 0 || z < 0 || s[a+len(start):z] != CapabilityMatrix() {
		t.Fatal("README capability matrix differs from provider contracts; regenerate")
	}
}
