package docker

import (
	"fmt"
	"os"
	"strings"
)

// CheckSocket looks at the Docker socket before the agent talks to it and returns
// problems in plain words. An empty list means it looks fine. Hosts that aren't a
// unix socket (tcp://, ssh://) are not checked.
func CheckSocket(host string) []string {
	if !strings.HasPrefix(host, "unix://") {
		return nil
	}
	path := strings.TrimPrefix(host, "unix://")
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{fmt.Sprintf("Docker socket %s does not exist: is Docker installed and running?", path)}
		}
		return []string{fmt.Sprintf("cannot look at the Docker socket %s: %v", path, err)}
	}
	var problems []string
	if info.Mode()&os.ModeSocket == 0 {
		problems = append(problems, fmt.Sprintf("%s exists but is not a socket", path))
	}
	if info.Mode().Perm()&0o002 != 0 {
		problems = append(problems, fmt.Sprintf("%s is writable by everyone: any user on this server could control Docker (and so the whole server); tighten it to group docker", path))
	}
	return problems
}
