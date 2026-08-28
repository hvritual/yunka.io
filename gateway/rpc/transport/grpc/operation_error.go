package grpc

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"yunka.io/framework/execution"
	"yunka.io/framework/operation"
	"yunka.io/gateway/authz"
)

// OperationError maps framework/security execution failures to transport
// semantics while preserving application-owned errors unchanged.
func OperationError(err error) error {
	if err == nil {
		return nil
	}
	if authz.IsDenied(err) {
		return securityStatus(err)
	}
	if errors.Is(err, execution.ErrIdempotencyKeyRequired) {
		return status.Error(codes.InvalidArgument, "idempotency key required")
	}
	if errors.Is(err, execution.ErrIdempotencyInProgress) {
		return status.Error(codes.Aborted, "idempotent operation in progress")
	}
	if errors.Is(err, execution.ErrIdempotencyCompleted) {
		return status.Error(codes.AlreadyExists, "idempotent operation already completed")
	}
	if errors.Is(err, operation.ErrExecutorUnavailable) ||
		errors.Is(err, operation.ErrSecurityUnavailable) ||
		errors.Is(err, operation.ErrSecurityNilContext) ||
		errors.Is(err, operation.ErrIdempotencyUnavailable) {
		return status.Error(codes.Internal, "operation execution unavailable")
	}
	return err
}
