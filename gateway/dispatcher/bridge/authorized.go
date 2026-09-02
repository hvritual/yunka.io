package bridge

import (
	"errors"

	"github.com/hvritual/yunka.io/framework/core/identity"
	"github.com/hvritual/yunka.io/framework/core/request"
	"github.com/hvritual/yunka.io/gateway/authz"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
)

var (
	ErrAuthorizedExecutorTargetUnavailable = errors.New("gateway authorized executor: target executor is required")
	ErrAuthorizerUnavailable               = errors.New("gateway authorized executor: authorizer is required")
)

type AuthorizedExecutor struct {
	next       Executor
	authorizer authz.Authorizer
}

func NewAuthorizedExecutor(next Executor, authorizer authz.Authorizer) (*AuthorizedExecutor, error) {
	if next == nil {
		return nil, ErrAuthorizedExecutorTargetUnavailable
	}
	if authorizer == nil {
		return nil, ErrAuthorizerUnavailable
	}
	return &AuthorizedExecutor{next: next, authorizer: authorizer}, nil
}

func (executor *AuthorizedExecutor) Do(modName, srvName string, rt *request.Context, api *meta.RuntimeApi) ([]byte, error) {
	if executor == nil || executor.next == nil || executor.authorizer == nil {
		return nil, ErrAuthorizerUnavailable
	}
	policy := authz.PolicyFromRuntimeAPI(api)
	if !policy.Protected() {
		return executor.next.Do(modName, srvName, rt, api)
	}
	principal, _ := identity.FromContext(rt)
	decision, err := executor.authorizer.Authorize(rt, principal, policy)
	if err != nil {
		return nil, err
	}
	if !decision.Allowed {
		return nil, authz.Denied(decision)
	}
	return executor.next.Do(modName, srvName, rt, api)
}
