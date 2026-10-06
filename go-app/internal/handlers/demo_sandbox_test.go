package handlers

import (
	"strings"
	"testing"

	"docklite-agent/internal/demo"
	"docklite-agent/internal/testhelpers"
)

func TestDemoRootHelperNeverTouchesHost(t *testing.T) {
	old := demo.On
	demo.On = true
	defer func() { demo.On = old }()

	// nginx actions "succeed" in memory.
	_, err := runRootHelper(strings.NewReader("server {}"), "site-write", "demo.example.com")
	testhelpers.AssertNoError(t, err)
	_, err = runRootHelper(nil, "nginx-test")
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, demoConfigs["demo.example.com"], "server {}")
	// Anything that could reach the real host is refused.
	for _, cmd := range []string{"cert-issue", "cert-obtain", "cert-delete", "group-add", "group-remove", "rm-rf"} {
		_, err = runRootHelper(nil, cmd, "x")
		if err == nil {
			t.Fatalf("%s should be refused in demo mode", cmd)
		}
	}
	testhelpers.AssertEqual(t, hostName(), "demo-server")
}
