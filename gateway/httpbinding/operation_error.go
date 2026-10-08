package httpbinding

import (
	"errors"
	"net/http"

	"github.com/hvritual/yunka.io/framework/execution"
	"github.com/hvritual/yunka.io/framework/operation"
	"github.com/hvritual/yunka.io/gateway/authz"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WriteOperationError projects an already-executed Operation failure to a
// bounded HTTP response. The caller must pass only errors from the canonical
// Executor/Application path, never untrusted client-supplied status messages.
// It does not authorize a request, interpret route names, or change gRPC errors.
// Only fixed response messages are emitted; error.Error and gRPC status details
// must never reach the HTTP client.
func WriteOperationError(writer http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	code, message := classifyOperationError(err)
	http.Error(writer, message, code)
}

// classifyOperationError is the single status/error-text owner for C9 REST.
// Security and execution sentinels take precedence over application statuses.
// Unknown application errors intentionally retain the historical HTTP 400
// fallback rather than broadening the public status contract in this change.
func classifyOperationError(err error) (int, string) {
	if authz.IsDenied(err) {
		var denied *authz.DeniedError
		if errors.As(err, &denied) && denied != nil &&
			(denied.Decision.Reason == authz.ReasonUnauthenticated ||
				denied.Decision.Reason == authz.ReasonAuthenticationMethod) {
			return http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized)
		}
		return http.StatusForbidden, http.StatusText(http.StatusForbidden)
	}
	if errors.Is(err, execution.ErrIdempotencyKeyRequired) {
		return http.StatusBadRequest, "idempotency key required"
	}
	if errors.Is(err, execution.ErrIdempotencyInProgress) ||
		errors.Is(err, execution.ErrIdempotencyCompleted) {
		return http.StatusConflict, "idempotency conflict"
	}
	if errors.Is(err, operation.ErrExecutorUnavailable) ||
		errors.Is(err, operation.ErrSecurityUnavailable) ||
		errors.Is(err, operation.ErrSecurityNilContext) ||
		errors.Is(err, operation.ErrIdempotencyUnavailable) {
		return http.StatusInternalServerError, "operation execution unavailable"
	}
	switch status.Code(err) {
	case codes.NotFound:
		return http.StatusNotFound, "application not found"
	case codes.Aborted, codes.AlreadyExists:
		return http.StatusConflict, "application conflict"
	default:
		return http.StatusBadRequest, "application request failed"
	}
}
