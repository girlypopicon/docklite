package api

import (
	"net/http"
	"strings"

	hnd "docklite-agent/internal/handlers"
)

func NewRouter(handlers *hnd.Handlers, nextjsURL string) http.Handler {
	mux := http.NewServeMux()

	// Agent-handled API routes (Docker operations)
	mux.HandleFunc("/api/health", handlers.Auth(handlers.Health))
	mux.HandleFunc("/api/status", handlers.Auth(handlers.Status))
	mux.HandleFunc("/api/summary", handlers.Auth(handlers.Summary))
	mux.HandleFunc("/api/containers", handlers.Auth(handlers.ListContainers))
	mux.HandleFunc("/api/containers/all", handlers.Auth(handlers.ListAllContainers))
	mux.HandleFunc("/api/containers/scan", handlers.Auth(hnd.CSRFMiddleware(handlers.ScanSites)))
	mux.HandleFunc("/api/containers/onboard", handlers.Auth(hnd.CSRFMiddleware(handlers.OnboardSite)))
	mux.HandleFunc("/api/containers/import", handlers.Auth(hnd.CSRFMiddleware(handlers.ImportSite)))
	mux.HandleFunc("/api/containers/", handlers.Auth(hnd.CSRFMiddleware(handlers.Container)))
	mux.HandleFunc("/api/databases", handlers.Auth(handlers.Databases))
	mux.HandleFunc("/api/databases/stats", handlers.Auth(handlers.DatabaseStats))
	mux.HandleFunc("/api/databases/", handlers.Auth(hnd.CSRFMiddleware(handlers.DatabaseRoutes)))
	mux.HandleFunc("/api/files", handlers.Auth(handlers.Files))
	mux.HandleFunc("/api/files/content", handlers.Auth(handlers.FileContent))
	mux.HandleFunc("/api/files/create", handlers.Auth(hnd.CSRFMiddleware(handlers.CreatePath)))
	mux.HandleFunc("/api/files/delete", handlers.Auth(hnd.CSRFMiddleware(handlers.DeletePath)))
	mux.HandleFunc("/api/files/rename", handlers.Auth(hnd.CSRFMiddleware(handlers.RenamePath)))
	mux.HandleFunc("/api/files/upload", handlers.Auth(hnd.CSRFMiddleware(handlers.UploadFile)))
	mux.HandleFunc("/api/files/download", handlers.Auth(handlers.DownloadFile))
	mux.HandleFunc("/api/files/transfer", handlers.Auth(hnd.CSRFMiddleware(handlers.TransferFile)))
	mux.HandleFunc("/api/files/folder", handlers.Auth(hnd.CSRFMiddleware(handlers.DeleteFolder)))
	mux.HandleFunc("/api/server/stats", handlers.Auth(handlers.ServerStats))
	mux.HandleFunc("/api/server/overview", handlers.Auth(handlers.ServerOverview))
	mux.HandleFunc("/api/server/updates", handlers.Auth(handlers.ServerUpdates))
	mux.HandleFunc("/api/server/services", handlers.Auth(handlers.ServerServices))
	mux.HandleFunc("/api/server/services/action", handlers.Auth(hnd.CSRFMiddleware(handlers.ServerServiceAction)))
	mux.HandleFunc("/api/server/storage", handlers.Auth(handlers.ServerStorage))
	mux.HandleFunc("/api/server/storage/prune", handlers.Auth(hnd.CSRFMiddleware(handlers.ServerStoragePrune)))
	mux.HandleFunc("/api/server/security", handlers.Auth(handlers.ServerSecurity))
	mux.HandleFunc("/api/server/logs", handlers.Auth(handlers.ServerLogs))
	mux.HandleFunc("/api/server/diagnostics", handlers.Auth(handlers.ServerDiagnostics))
	mux.HandleFunc("/api/ports/suggest", handlers.Auth(handlers.SuggestPort))
	mux.HandleFunc("/api/network/overview", handlers.Auth(handlers.NetworkOverview))
	mux.HandleFunc("/api/network/firewall", handlers.Auth(hnd.CSRFMiddleware(handlers.NetworkFirewall)))
	mux.HandleFunc("/api/network/ingress", handlers.Auth(handlers.NetworkIngress))
	mux.HandleFunc("/api/network/public-ip", handlers.Auth(handlers.NetworkPublicIP))
	mux.HandleFunc("/api/network/diagnostics", handlers.Auth(handlers.NetworkDiagnostics))
	mux.HandleFunc("/api/folders", handlers.Auth(hnd.CSRFMiddleware(handlers.Folders)))
	mux.HandleFunc("/api/folders/", handlers.Auth(hnd.CSRFMiddleware(handlers.FolderRoutes)))
	mux.HandleFunc("/api/backups", handlers.Auth(hnd.CSRFMiddleware(handlers.Backups)))
	mux.HandleFunc("/api/backups/destinations", handlers.Auth(hnd.CSRFMiddleware(handlers.BackupDestinations)))
	mux.HandleFunc("/api/backups/jobs", handlers.Auth(hnd.CSRFMiddleware(handlers.BackupJobs)))
	mux.HandleFunc("/api/backups/history", handlers.Auth(handlers.BackupHistory))
	mux.HandleFunc("/api/backups/local", handlers.Auth(handlers.LocalBackups))
	mux.HandleFunc("/api/backups/local/download", handlers.Auth(handlers.LocalBackupDownload))
	mux.HandleFunc("/api/backups/trigger", handlers.Auth(hnd.CSRFMiddleware(handlers.BackupTrigger)))
	mux.HandleFunc("/api/backups/export", handlers.Auth(hnd.CSRFMiddleware(handlers.BackupExport)))
	mux.HandleFunc("/api/dns/config", handlers.Auth(hnd.CSRFMiddleware(handlers.DNSConfig)))
	mux.HandleFunc("/api/dns/zones", handlers.Auth(handlers.DNSZones))
	mux.HandleFunc("/api/dns/records", handlers.Auth(hnd.CSRFMiddleware(handlers.DNSRecords)))
	mux.HandleFunc("/api/dns/sync", handlers.Auth(hnd.CSRFMiddleware(handlers.DNSSync)))
	mux.HandleFunc("/api/ssl", handlers.Auth(handlers.SSLBasic))
	mux.HandleFunc("/api/ssl/status", handlers.Auth(handlers.SSLStatus))
	mux.HandleFunc("/api/ssl/issue", handlers.Auth(hnd.CSRFMiddleware(handlers.SSLIssue)))
	mux.HandleFunc("/api/ssl/renew", handlers.Auth(hnd.CSRFMiddleware(handlers.SSLRenew)))
	mux.HandleFunc("/api/ssl/delete", handlers.Auth(hnd.CSRFMiddleware(handlers.SSLDelete)))
	mux.HandleFunc("/api/ssl/repair", handlers.Auth(hnd.CSRFMiddleware(handlers.SSLRepair)))
	mux.HandleFunc("/api/users", handlers.Auth(hnd.CSRFMiddleware(handlers.Users)))
	mux.HandleFunc("/api/users/password", handlers.Auth(hnd.CSRFMiddleware(handlers.UserPassword)))
	mux.HandleFunc("/api/system/check-folders", handlers.Auth(handlers.SystemCheckFolders))
	mux.HandleFunc("/api/system/update/status", handlers.Auth(handlers.SystemUpdateStatus))
	mux.HandleFunc("/api/system/update/run", handlers.Auth(hnd.CSRFMiddleware(handlers.SystemUpdateRun)))
	mux.HandleFunc("/api/db/cleanup", handlers.Auth(hnd.CSRFMiddleware(handlers.DBCleanup)))
	mux.HandleFunc("/api/db", handlers.Auth(hnd.CSRFMiddleware(handlers.DBDebug)))
	mux.HandleFunc("/api/debug", handlers.Auth(handlers.Debug))
	mux.HandleFunc("/api/tokens", handlers.Auth(hnd.CSRFMiddleware(handlers.Tokens)))
	mux.HandleFunc("/api/tokens/revoke", handlers.Auth(hnd.CSRFMiddleware(handlers.TokenRevoke)))
	mux.HandleFunc("/api/auth/login", handlers.AuthLogin)
	mux.HandleFunc("/api/auth/me", handlers.Auth(handlers.AuthMe))
	mux.HandleFunc("/api/auth/logout", handlers.Auth(hnd.CSRFMiddleware(handlers.AuthLogout)))
	mux.HandleFunc("/api/csrf-token", handlers.Auth(handlers.CSRFToken))

	// Proxy for Next.js (optional)
	var proxy http.Handler
	if nextjsURL != "" {
		proxy = ProxyHandler(nextjsURL)
	}

	// Wrap the mux with a handler that proxies non-API routes
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this is an agent-handled route
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			mux.ServeHTTP(w, r)
		} else if proxy != nil {
			// Proxy everything else to Next.js
			proxy.ServeHTTP(w, r)
		} else {
			http.NotFound(w, r)
		}
	})
}
