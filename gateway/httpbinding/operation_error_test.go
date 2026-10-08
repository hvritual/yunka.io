package httpbinding

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/framework/execution"
	"github.com/hvritual/yunka.io/framework/operation"
	"github.com/hvritual/yunka.io/gateway/authz"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Every case is an externally observable REST response. This table is the
// authority for the bounded C9 error mapping; generated adapters only call it.
func TestWriteOperationErrorPreservesFrameworkPriorityAndSafeStatuses(t *testing.T) {
	secret := "secret-tenant-identifier-must-not-leak"
	cases := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{
			name: "authenticated policy denial", err: authz.Denied(authz.Decision{
				Operation: "protected.operation", Reason: authz.ReasonPermissionDenied,
			}), status: http.StatusForbidden, message: "Forbidden",
		},
		{
			name: "unauthenticated policy denial", err: authz.Denied(authz.Decision{
				Operation: "protected.operation", Reason: authz.ReasonUnauthenticated,
			}), status: http.StatusUnauthorized, message: "Unauthorized",
		},
		{
			name: "wrong authentication method", err: fmt.Errorf("%s: %w", secret,
				authz.Denied(authz.Decision{Reason: authz.ReasonAuthenticationMethod})),
			status: http.StatusUnauthorized, message: "Unauthorized",
		},
		{
			name: "bare policy denied sentinel", err: authz.ErrDenied,
			status: http.StatusForbidden, message: "Forbidden",
		},
		{
			name: "idempotency key missing", err: execution.ErrIdempotencyKeyRequired,
			status: http.StatusBadRequest, message: "idempotency key required",
		},
		{
			name: "idempotent operation in progress", err: fmt.Errorf("%s: %w", secret,
				execution.ErrIdempotencyInProgress),
			status: http.StatusConflict, message: "idempotency conflict",
		},
		{
			name: "idempotent operation completed", err: execution.ErrIdempotencyCompleted,
			status: http.StatusConflict, message: "idempotency conflict",
		},
		{
			name: "executor unavailable", err: operation.ErrExecutorUnavailable,
			status: http.StatusInternalServerError, message: "operation execution unavailable",
		},
		{
			name: "execution security unavailable", err: operation.ErrSecurityUnavailable,
			status: http.StatusInternalServerError, message: "operation execution unavailable",
		},
		{
			name: "missing trusted execution scope", err: operation.ErrSecurityNilContext,
			status: http.StatusInternalServerError, message: "operation execution unavailable",
		},
		{
			name: "idempotency store unavailable", err: fmt.Errorf("%s: %w", secret,
				operation.ErrIdempotencyUnavailable),
			status: http.StatusInternalServerError, message: "operation execution unavailable",
		},
		{
			name: "application not found", err: status.Error(codes.NotFound, secret),
			status: http.StatusNotFound, message: "application not found",
		},
		{
			name: "wrapped application not found", err: fmt.Errorf("wrapped: %w",
				status.Error(codes.NotFound, secret)),
			status: http.StatusNotFound, message: "application not found",
		},
		{
			name: "application aborted", err: status.Error(codes.Aborted, secret),
			status: http.StatusConflict, message: "application conflict",
		},
		{
			name: "application already exists", err: status.Error(codes.AlreadyExists, secret),
			status: http.StatusConflict, message: "application conflict",
		},
		{
			name: "application invalid argument", err: status.Error(codes.InvalidArgument, secret),
			status: http.StatusBadRequest, message: "application request failed",
		},
		{
			name: "application permission denied is not authorization", err: status.Error(codes.PermissionDenied, secret),
			status: http.StatusBadRequest, message: "application request failed",
		},
		{
			name: "application unauthenticated is not authorization", err: status.Error(codes.Unauthenticated, secret),
			status: http.StatusBadRequest, message: "application request failed",
		},
		{
			name: "application internal is not framework sentinel", err: status.Error(codes.Internal, secret),
			status: http.StatusBadRequest, message: "application request failed",
		},
		{
			name: "unknown Go error", err: errors.New(secret),
			status: http.StatusBadRequest, message: "application request failed",
		},
		{
			name: "error message that resembles grpc status", err: errors.New("rpc error: code = NotFound desc = " + secret),
			status: http.StatusBadRequest, message: "application request failed",
		},
		{
			name: "execution sentinel wins over wrapped application status",
			err: errors.Join(operation.ErrExecutorUnavailable, status.Error(codes.NotFound, secret)),
			status: http.StatusInternalServerError, message: "operation execution unavailable",
		},
		{
			name: "authentication authority wins over application status",
			err: errors.Join(authz.Denied(authz.Decision{Reason: authz.ReasonUnauthenticated}),
				status.Error(codes.NotFound, secret)),
			status: http.StatusUnauthorized, message: "Unauthorized",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			writer := httptest.NewRecorder()
			WriteOperationError(writer, test.err)
			if writer.Code != test.status {
				t.Fatalf("HTTP status=%d, want %d", writer.Code, test.status)
			}
			if got := writer.Body.String(); got != test.message+"\n" {
				t.Fatalf("unsafe or unstable HTTP response body %q, want %q", got, test.message+"\n")
			}
			if got := writer.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
				t.Fatalf("HTTP content type changed: %q", got)
			}
			if got := writer.Body.String(); strings.Contains(got, secret) {
				t.Fatalf("internal application/executor status detail leaked: %q", got)
			}
		})
	}
}

func TestWriteOperationErrorNilIsNotAnErrorResponse(t *testing.T) {
	writer := httptest.NewRecorder()
	WriteOperationError(writer, nil)
	if writer.Body.Len() != 0 || writer.Header().Get("Content-Type") != "" {
		t.Fatalf("a successful Operation cannot emit an error response")
	}
}
