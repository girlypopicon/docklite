package handlers

import (
	"context"
	"net/http"

	"docklite-agent/internal/docker"
	"docklite-agent/internal/store"
)

type Handlers struct {
	docker        *docker.Client
	store         *store.SQLiteStore
	token         string
	backupBaseDir string
	sslManager    *SSLManager
	listenAddr    string
	nextjsURL     string
}

func New(dockerClient *docker.Client, store *store.SQLiteStore, token string, backupBaseDir string, listenAddr string, nextjsURL string) *Handlers {
	return &Handlers{
		docker:        dockerClient,
		store:         store,
		token:         token,
		backupBaseDir: backupBaseDir,
		listenAddr:    listenAddr,
		nextjsURL:     nextjsURL,
	}
}

func (h *Handlers) Auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authToken, err := parseBearerToken(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if authToken != "" {
			record, err := h.authenticateBearer(r.Context(), authToken)
			if err == nil && record != nil {
				ctx := r.Context()
				if record.UserID != nil {
					ctx = context.WithValue(ctx, ctxUserIDKey, *record.UserID)
				}
				role := ""
				if record.Role != nil {
					role = *record.Role
				}
				if role == "" && record.UserID != nil {
					if user, err := h.store.GetUserByIDFull(*record.UserID); err == nil && user != nil {
						role = user.Role
					}
				}
				if role != "" {
					ctx = context.WithValue(ctx, ctxUserRoleKey, role)
				}
				next(w, r.WithContext(ctx))
				return
			}
		}

		if h.token != "" {
			if r2, ok := withDelegationContext(r, h.token); ok {
				next(w, r2)
				return
			}
		}
		writeError(w, http.StatusUnauthorized, "unauthorized")
	}
}
