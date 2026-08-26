package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/hvritual/biz/internal/device/operations"
	"github.com/hvritual/biz/internal/iam/access"
	"gorm.io/gorm"
	"yunka.io/framework/core/identity"
)

type accessContextKey struct{}

type Handler struct {
	service  *operations.Service
	auth     *access.Authenticator
	database *gorm.DB
	mux      *http.ServeMux
}

func NewHandler(service *operations.Service, auth *access.Authenticator, database *gorm.DB) http.Handler {
	handler := &Handler{service: service, auth: auth, database: database, mux: http.NewServeMux()}
	handler.mux.HandleFunc("/healthz", handler.health)
	handler.mux.Handle("/v1/me", handler.requireAuth(http.HandlerFunc(handler.me)))
	handler.mux.Handle("/v1/devices", handler.requireAuth(http.HandlerFunc(handler.devices)))
	handler.mux.Handle("/v1/devices/", handler.requireAuth(http.HandlerFunc(handler.device)))
	return handler.mux
}

func (handler *Handler) health(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	sqlDB, err := handler.database.DB()
	if err != nil || sqlDB.PingContext(request.Context()) != nil {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]any{"status": "unhealthy"})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"status": "ok"})
}

func (handler *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		header := strings.TrimSpace(request.Header.Get("Authorization"))
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(writer, access.ErrUnauthorized)
			return
		}
		plan, err := handler.auth.Authenticate(request.Context(), strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
		if err != nil {
			writeError(writer, err)
			return
		}
		ctx := identity.WithPrincipal(request.Context(), plan.Principal)
		ctx = context.WithValue(ctx, accessContextKey{}, plan)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func planFrom(request *http.Request) (access.Plan, bool) {
	plan, ok := request.Context().Value(accessContextKey{}).(access.Plan)
	return plan, ok
}

func (handler *Handler) me(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	plan, ok := planFrom(request)
	if !ok {
		writeError(writer, access.ErrUnauthorized)
		return
	}
	type grant struct {
		Permission string `json:"permission"`
		All        bool   `json:"all"`
		Sites      bool   `json:"sites"`
		Self       bool   `json:"self"`
	}
	grants := make([]grant, 0, len(plan.Permissions))
	for permission, scope := range plan.Permissions {
		grants = append(grants, grant{Permission: permission, All: scope.All, Sites: scope.Sites, Self: scope.Self})
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"tenantId": plan.Principal.TenantID,
		"userId":   plan.Principal.UserID,
		"roles":    plan.Principal.Roles,
		"siteIds":  plan.SiteIDs,
		"grants":   grants,
	})
}

func (handler *Handler) devices(writer http.ResponseWriter, request *http.Request) {
	plan, ok := planFrom(request)
	if !ok {
		writeError(writer, access.ErrUnauthorized)
		return
	}
	switch request.Method {
	case http.MethodGet:
		devices, err := handler.service.ListDevices(request.Context(), plan)
		if err != nil {
			writeError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, devices)
	case http.MethodPost:
		var input operations.CreateDeviceInput
		if err := decodeJSON(request, &input); err != nil {
			writeError(writer, operations.ErrInvalid)
			return
		}
		device, err := handler.service.CreateDevice(request.Context(), plan, input)
		if err != nil {
			writeError(writer, err)
			return
		}
		writeJSON(writer, http.StatusCreated, device)
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (handler *Handler) device(writer http.ResponseWriter, request *http.Request) {
	plan, ok := planFrom(request)
	if !ok {
		writeError(writer, access.ErrUnauthorized)
		return
	}
	id := strings.TrimSpace(strings.TrimPrefix(request.URL.Path, "/v1/devices/"))
	if id == "" || strings.Contains(id, "/") {
		writeError(writer, operations.ErrNotFound)
		return
	}
	switch request.Method {
	case http.MethodGet:
		device, err := handler.service.GetDevice(request.Context(), plan, id)
		if err != nil {
			writeError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, device)
	case http.MethodPatch:
		var input operations.UpdateDeviceInput
		if err := decodeJSON(request, &input); err != nil {
			writeError(writer, operations.ErrInvalid)
			return
		}
		device, err := handler.service.UpdateDevice(request.Context(), plan, id, input)
		if err != nil {
			writeError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, device)
	case http.MethodDelete:
		version, err := strconv.ParseUint(request.URL.Query().Get("version"), 10, 64)
		if err != nil || version == 0 {
			writeError(writer, operations.ErrInvalid)
			return
		}
		if err := handler.service.DeleteDevice(request.Context(), plan, id, version); err != nil {
			writeError(writer, err)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func decodeJSON(request *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return operations.ErrInvalid
	}
	return nil
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	switch {
	case errors.Is(err, access.ErrUnauthorized):
		status, code = http.StatusUnauthorized, "unauthorized"
	case errors.Is(err, access.ErrForbidden):
		status, code = http.StatusForbidden, "forbidden"
	case errors.Is(err, operations.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, operations.ErrConflict):
		status, code = http.StatusConflict, "conflict"
	case errors.Is(err, operations.ErrInvalid):
		status, code = http.StatusBadRequest, "invalid_request"
	}
	writeJSON(writer, status, map[string]any{"error": code})
}
