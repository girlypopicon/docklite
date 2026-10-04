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
				userID := record.UserID
				if userID == nil && record.Role != nil && (*record.Role == "super_admin" || *record.Role == "admin") {
					// The bootstrap token can predate the first user; once a
					// super admin exists, the token acts as them.
					if owner, err := h.store.GetUserByRole("super_admin"); err == nil && owner != nil {
						userID = &owner.ID
						_ = h.store.LinkTokenUser(record.ID, owner.ID)
					}
				}
				if userID != nil {
					ctx = context.WithValue(ctx, ctxUserIDKey, *userID)
				}
				role := ""
				if record.Role != nil {
					role = *record.Role
				}
				if role == "" && userID != nil {
					if user, err := h.store.GetUserByIDFull(*userID); err == nil && user != nil {
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
				// The cookie's role is a snapshot from login. Re-read the user
				// so deleted or demoted accounts lose access immediately.
				userID, _ := readUserIDFromContext(r2)
				user, err := h.store.GetUserByIDFull(userID)
				if err != nil || user == nil {
					writeError(w, http.StatusUnauthorized, "unauthorized")
					return
				}
				ctx := context.WithValue(r2.Context(), ctxUserRoleKey, normalizeUserRole(user))
				next(w, r2.WithContext(ctx))
				return
			}
		}
		writeError(w, http.StatusUnauthorized, "unauthorized")
	}
}
