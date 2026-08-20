package identity

import "context"

const (
	AuthMethodJWT          = "jwt"
	AuthMethodAPIKey       = "api-key"
	AuthMethodServiceToken = "service-token"
)

// Principal is the trusted caller identity attached to a runtime context.
// Authenticated must only be set after server-side credential validation.
type Principal struct {
	Subject       string
	TenantID      string
	UserID        string
	Roles         []string
	AuthMethod    string
	Authenticated bool
}

func (p Principal) Clone() Principal {
	clone := p
	if p.Roles != nil {
		clone.Roles = append([]string(nil), p.Roles...)
	}
	return clone
}

func (p Principal) HasRole(role string) bool {
	for _, current := range p.Roles {
		if current == role {
			return true
		}
	}
	return false
}

type principalContextKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, principalContextKey{}, principal.Clone())
}

func FromContext(ctx context.Context) (Principal, bool) {
	if ctx == nil {
		return Principal{}, false
	}
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	if !ok {
		return Principal{}, false
	}
	return principal.Clone(), true
}
