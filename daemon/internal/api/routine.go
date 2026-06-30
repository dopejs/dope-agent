package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/routine"
)

// RoutineRequest creates or previews a routine (Roadmap 66). It is explicit configuration.
type RoutineRequest struct {
	Definition routine.Definition `json:"definition"`
}

type RoutineListResponse struct {
	Items []routine.Routine `json:"items"`
}

func handleRoutines(manager *routine.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "routine manager is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, RoutineListResponse{Items: manager.List()})
	case http.MethodPost:
		var request RoutineRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		created, err := manager.Create(r.Context(), request.Definition)
		if err != nil {
			writeRoutineError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleRoutineRoutes(manager *routine.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "routine manager is not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/routines/")
	parts := strings.Split(path, "/")
	routineID := strings.TrimSpace(parts[0])
	if routineID == "" {
		http.NotFound(w, r)
		return
	}
	// Preview is a dry-run that does not reference an existing routine.
	if routineID == "preview" && r.Method == http.MethodPost {
		var request RoutineRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		preview, err := manager.Preview(request.Definition)
		if err != nil {
			writeRoutineError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, preview)
		return
	}

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			routineItem, ok := manager.Get(routineID)
			if !ok {
				writeRoutineError(w, routine.ErrRoutineNotFound)
				return
			}
			writeJSON(w, http.StatusOK, routineItem)
		case http.MethodPut:
			var request RoutineRequest
			if err := decodeJSONBody(r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			updated, err := manager.Update(r.Context(), routineID, request.Definition)
			if err != nil {
				writeRoutineError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, updated)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	if len(parts) == 2 && r.Method == http.MethodPost {
		var (
			result routine.Routine
			err    error
		)
		switch parts[1] {
		case "pause":
			result, err = manager.Pause(r.Context(), routineID)
		case "resume":
			result, err = manager.Resume(r.Context(), routineID)
		case "cancel":
			result, err = manager.Cancel(r.Context(), routineID)
		case "repair":
			result, err = manager.Repair(r.Context(), routineID)
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			writeRoutineError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	http.NotFound(w, r)
}

func writeRoutineError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, routine.ErrRoutineNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, routine.ErrInvalidRoutine), errors.Is(err, routine.ErrRoutineCancelled):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
