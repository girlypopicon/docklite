package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Audit trail for security-sensitive actions: who did what, to what, from
// where. One JSON object per line in logs/audit.log next to the other logs.
// It is best effort — a failure to write is logged but never blocks the action.

var auditMu sync.Mutex

type auditEntry struct {
	Time   string         `json:"time"`
	Actor  int64          `json:"actorId"`
	Role   string         `json:"actorRole"`
	From   string         `json:"from"`
	Action string         `json:"action"`
	Target string         `json:"target"`
	Detail map[string]any `json:"detail,omitempty"`
}

func (h *Handlers) audit(r *http.Request, action, target string, detail map[string]any) {
	actor, _ := readUserIDFromContext(r)
	role, _ := readUserRoleFromContext(r)
	entry := auditEntry{
		Time:   time.Now().UTC().Format(time.RFC3339),
		Actor:  actor,
		Role:   role,
		From:   clientIP(r),
		Action: action,
		Target: target,
		Detail: detail,
	}
	appendAudit(entry)
}

// auditSystem records something DockLite did by itself (no signed-in user), e.g. an automatic fix.
func auditSystem(action, target string, detail map[string]any) {
	appendAudit(auditEntry{Time: time.Now().UTC().Format(time.RFC3339), Role: "system", From: "agent", Action: action, Target: target, Detail: detail})
}

func appendAudit(entry auditEntry) {
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}
	auditMu.Lock()
	defer auditMu.Unlock()
	path := filepath.Join("logs", "audit.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		log.Printf("audit: %v", err)
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		log.Printf("audit: %v", err)
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}
