package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/catalog"
)

// RegisterCatalogItemRequest registers an operator-curated catalog item (Roadmap 68).
type RegisterCatalogItemRequest struct {
	Item catalog.CatalogItem `json:"item"`
}

// CatalogEnablementRequest enables/disables/rolls back a catalog item for a tenant.
type CatalogEnablementRequest struct {
	TenantID string `json:"tenantId,omitempty"`
	Version  string `json:"version,omitempty"`
	Actor    string `json:"actor,omitempty"`
}

type CatalogItemListResponse struct {
	Items []catalog.CatalogItem `json:"items"`
}

func catalogTenant(r *http.Request, bodyTenant string) string {
	if t := strings.TrimSpace(bodyTenant); t != "" {
		return t
	}
	return strings.TrimSpace(r.URL.Query().Get("tenantId"))
}

func handleCatalogItems(manager *catalog.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "catalog manager is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, CatalogItemListResponse{Items: manager.ListItems()})
	case http.MethodPost:
		var request RegisterCatalogItemRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		item, err := manager.RegisterItem(request.Item)
		if err != nil {
			writeCatalogError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleCatalogItemRoutes(manager *catalog.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "catalog manager is not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/catalog/items/")
	parts := strings.Split(path, "/")
	itemID := strings.TrimSpace(parts[0])
	if itemID == "" {
		http.NotFound(w, r)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodGet {
		inspection, err := manager.Inspect(r.Context(), catalogTenant(r, ""), itemID)
		if err != nil {
			writeCatalogError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, inspection)
		return
	}
	if len(parts) == 2 && r.Method == http.MethodPost {
		var request CatalogEnablementRequest
		if err := decodeJSONBody(r, &request); err != nil && !strings.Contains(err.Error(), "EOF") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		tenant := catalogTenant(r, request.TenantID)
		var (
			enablement catalog.Enablement
			err        error
		)
		switch parts[1] {
		case "enable":
			enablement, err = manager.Enable(r.Context(), tenant, itemID, strings.TrimSpace(request.Version), strings.TrimSpace(request.Actor))
		case "disable":
			enablement, err = manager.Disable(r.Context(), tenant, itemID, strings.TrimSpace(request.Actor))
		case "rollback":
			enablement, err = manager.Rollback(r.Context(), tenant, itemID, strings.TrimSpace(request.Actor))
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			writeCatalogError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, enablement)
		return
	}
	http.NotFound(w, r)
}

func writeCatalogError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, catalog.ErrItemNotFound), errors.Is(err, catalog.ErrVersionNotFound), errors.Is(err, catalog.ErrNoRollbackTarget):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, catalog.ErrPermissionDenied):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, catalog.ErrRequirementsUnmet), errors.Is(err, catalog.ErrInvalidCatalogItem):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
