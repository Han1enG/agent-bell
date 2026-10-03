package main

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/han1eng/agent-bell/internal/surface"
)

func TestCurrentContextInstalledIsNotActive(t *testing.T) {
	for _, name := range []string{"tabby", "jetbrains", "generic"} {
		var out bytes.Buffer
		printCurrentContext(&out, surface.ReturnTarget{Surface: name, AppBundleID: "app"}, func(string) string { return "" }, func(surface.ReturnTarget) error { t.Fatal("missing identity was probed"); return nil })
		if !strings.Contains(out.String(), "App return available") || !strings.Contains(out.String(), "unavailable") || strings.Contains(out.String(), "✓ Exact") {
			t.Fatal(out.String())
		}
	}
}
func TestCurrentContextLiveProbeRequired(t *testing.T) {
	for _, reason := range []surface.FailureReason{"", surface.ContextNotFound, surface.BridgeUnreachable, surface.PermissionDenied} {
		var out bytes.Buffer
		probed := false
		target := surface.ReturnTarget{Surface: "tabby", ContextID: "opaque", Capability: surface.ReturnExactContext}
		printCurrentContext(&out, target, func(k string) string {
			if k == "AGENTBELL_SURFACE" {
				return "tabby"
			}
			return "opaque"
		}, func(got surface.ReturnTarget) error {
			probed = true
			if !reflect.DeepEqual(got, target) {
				t.Fatal(got)
			}
			if reason == "" {
				return nil
			}
			return surface.Fail(reason, errors.New("fixture"))
		})
		if !probed || strings.Contains(out.String(), "✓ Exact") != (reason == "") {
			t.Fatal(out.String())
		}
		if reason != "" && !strings.Contains(out.String(), string(reason)) {
			t.Fatal(out.String())
		}
	}
}
