package handlers

import (
	"errors"
	"io"
	"os"
	"sync"

	"docklite-agent/internal/demo"
)

// In demo mode root actions never leave the process: nginx configs are kept in
// memory and certificates are simulated, so the demo can't change the real host.

var (
	demoMu      sync.Mutex
	demoConfigs = map[string]string{}
)

var errDemoDisabled = errors.New("this isn't available in demo mode")

func demoRootHelper(stdin io.Reader, args ...string) ([]byte, error) {
	if len(args) == 0 {
		return nil, errDemoDisabled
	}
	demoMu.Lock()
	defer demoMu.Unlock()
	switch args[0] {
	case "site-write":
		if len(args) > 1 && stdin != nil {
			if data, err := io.ReadAll(stdin); err == nil {
				demoConfigs[args[1]] = string(data)
			}
		}
		return nil, nil
	case "site-enable", "site-disable", "nginx-test", "nginx-reload", "nginx-restart", "nginx-survey":
		return nil, nil
	case "site-remove":
		if len(args) > 1 {
			delete(demoConfigs, args[1])
		}
		return nil, nil
	case "cert-list", "group-list":
		return nil, nil
	default:
		// Certificates, user groups and anything else that would reach the real host.
		return []byte("disabled in demo mode"), errDemoDisabled
	}
}

// hostName is the machine's name, or a made-up one in demo mode.
func hostName() string {
	if demo.On {
		return "demo-server"
	}
	name, _ := os.Hostname()
	return name
}
