package grpc

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	if errors.Is(err, operation.ErrExecutorUnavailable) ||
		errors.Is(err, operation.ErrSecurityUnavailable) ||
		errors.Is(err, operation.ErrSecurityNilContext) {
		return status.Error(codes.Internal, "operation execution unavailable")
	}
	return err
}
