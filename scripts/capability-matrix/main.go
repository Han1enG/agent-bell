// Regenerate only the README capability block from trusted provider declarations.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/han1eng/agent-bell/internal/surface/builtin"
)

func main() {
	b, err := os.ReadFile("README.md")
	if err != nil {
		panic(err)
	}
	s := string(b)
	aMarker := "<!-- agentbell-capabilities:start -->\n"
	zMarker := "<!-- agentbell-capabilities:end -->"
	a := strings.Index(s, aMarker)
	z := strings.Index(s, zMarker)
	if a < 0 || z < a {
		panic("missing matrix markers")
	}
	s = s[:a+len(aMarker)] + builtin.CapabilityMatrix() + s[z:]
	if err := os.WriteFile("README.md", []byte(s), 0644); err != nil {
		panic(err)
	}
	fmt.Println("README capability matrix updated")
}
