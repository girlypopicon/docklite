package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

type terminalResizeMessage struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

var terminalUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		host := r.Host
		if host == "" {
			host = r.Header.Get("Host")
		}
		// Allow same-origin requests: origin must end with the host
		return strings.HasSuffix(origin, "://"+host)
	},
}

func (h *Handlers) ContainerTerminal(w http.ResponseWriter, r *http.Request, containerID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := dockerContext(r.Context())
	defer cancel()

	if _, err := h.authorizeContainerAccess(ctx, r, containerID); err != nil {
		if err == errForbidden {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	conn, err := terminalUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	attachResp, execID, err := h.docker.ExecShell(ctx, containerID)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("Failed to start shell session.\r\n"))
		return
	}
	defer attachResp.Close()

	var closeOnce sync.Once
	closeAll := func() {
		closeOnce.Do(func() {
			attachResp.Close()
			_ = conn.Close()
		})
	}

	go func() {
		buffer := make([]byte, 4096)
		for {
			n, readErr := attachResp.Reader.Read(buffer)
			if n > 0 {
				if writeErr := conn.WriteMessage(websocket.BinaryMessage, buffer[:n]); writeErr != nil {
					closeAll()
					return
				}
			}
			if readErr != nil {
				if readErr != io.EOF {
					closeAll()
				}
				return
			}
		}
	}()

	for {
		messageType, payload, readErr := conn.ReadMessage()
		if readErr != nil {
			closeAll()
			return
		}

		if messageType == websocket.TextMessage {
			var resize terminalResizeMessage
			if err := json.Unmarshal(payload, &resize); err == nil && resize.Type == "resize" {
				_ = h.docker.ResizeExec(ctx, execID, resize.Cols, resize.Rows)
				continue
			}
		}

		if messageType == websocket.BinaryMessage || messageType == websocket.TextMessage {
			if len(payload) == 0 {
				continue
			}
			if _, err := attachResp.Conn.Write(payload); err != nil {
				closeAll()
				return
			}
		}
	}
}
