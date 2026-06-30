package api

import (
	"net/http"

	"github.com/dopejs/dope-agent/daemon/internal/opsreadiness"
)

// handleLaunchGateValidate evaluates a posted public-beta launch-gate evidence index and returns
// the ship/no-ship decision (Roadmap 72). It is a pure validator over caller-supplied evidence;
// it owns no runtime truth.
func handleLaunchGateValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var evidence opsreadiness.LaunchGateEvidence
	if err := decodeJSONBody(r, &evidence); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, opsreadiness.ValidateLaunchGate(evidence))
}
