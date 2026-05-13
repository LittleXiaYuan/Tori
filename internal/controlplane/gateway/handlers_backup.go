package gateway

import (
	"net/http"

	backuppack "yunque-agent/internal/packs/backup"
)

// handleBackupExport delegates to the backup pack implementation. The route is
// mounted through Pack Runtime gating in handlers_packs.go, so this wrapper
// intentionally stays thin.
func (g *Gateway) handleBackupExport(w http.ResponseWriter, r *http.Request) {
	backuppack.DefaultHandler().Export(w, r)
}

// handleBackupImport delegates to the backup pack implementation.
func (g *Gateway) handleBackupImport(w http.ResponseWriter, r *http.Request) {
	backuppack.DefaultHandler().Import(w, r)
}

// handleBackupInfo delegates to the backup pack implementation.
func (g *Gateway) handleBackupInfo(w http.ResponseWriter, r *http.Request) {
	backuppack.DefaultHandler().Info(w, r)
}
