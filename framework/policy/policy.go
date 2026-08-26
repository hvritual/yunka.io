package policy

import (
	"context"
	"errors"
	"sort"
	"strings"

	"yunka.io/framework/core/identity"
)

var (
	ErrUnauthorized = errors.New("policy: unauthorized")
	ErrForbidden    = errors.New("policy: forbidden")
)

type Grant struct {
	Allowed bool
	All     bool
	Self    bool
	SiteIDs []string
}

type Resolver interface {
	Resolve(context.Context, identity.Principal, string) (Grant, error)
}

type ResolverFunc func(context.Context, identity.Principal, string) (Grant, error)

func (fn ResolverFunc) Resolve(ctx context.Context, principal identity.Principal, permission string) (Grant, error) {
	return fn(ctx, principal, permission)
}

type grantContextKey struct{}

func WithGrants(ctx context.Context, grants map[string]Grant) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	copyOf := make(map[string]Grant, len(grants))
	for permission, grant := range grants {
		grant.SiteIDs = normalizeIDs(grant.SiteIDs)
		copyOf[strings.TrimSpace(permission)] = grant
	}
	return context.WithValue(ctx, grantContextKey{}, copyOf)
}

func GrantsFromContext(ctx context.Context) (map[string]Grant, bool) {
	if ctx == nil {
		return nil, false
	}
	grants, ok := ctx.Value(grantContextKey{}).(map[string]Grant)
	return grants, ok
}

type ContextResolver struct{}

func (ContextResolver) Resolve(ctx context.Context, principal identity.Principal, permission string) (Grant, error) {
	if !principal.Authenticated {
		return Grant{}, ErrUnauthorized
	}
	grants, ok := GrantsFromContext(ctx)
	if !ok {
		return Grant{}, nil
	}
	grant, ok := grants[strings.TrimSpace(permission)]
	if !ok {
		return Grant{}, nil
	}
	return grant, nil
}

type Filter struct {
	All      bool
	UseSites bool
	SiteIDs  []string
	UseSelf  bool
	OwnerID  string
}

func (filter Filter) DenyAll() bool {
	return !filter.All && !filter.UseSites && !filter.UseSelf
}

func (filter Filter) Normalize() Filter {
	if filter.All {
		return Filter{All: true}
	}
	filter.SiteIDs = normalizeIDs(filter.SiteIDs)
	if len(filter.SiteIDs) == 0 {
		filter.UseSites = false
	}
	filter.OwnerID = strings.TrimSpace(filter.OwnerID)
	if filter.OwnerID == "" {
		filter.UseSelf = false
	}
	return filter
}

type Matcher[T any] interface {
	Allows(identity.Principal, Grant, T) bool
	Filter(identity.Principal, Grant) Filter
}

type matcher[T any] struct {
	allow  func(identity.Principal, Grant, T) bool
	filter func(identity.Principal, Grant) Filter
}

func (value matcher[T]) Allows(principal identity.Principal, grant Grant, target T) bool {
	return value.allow(principal, grant, target)
}

func (value matcher[T]) Filter(principal identity.Principal, grant Grant) Filter {
	return value.filter(principal, grant).Normalize()
}

func All[T any]() Matcher[T] {
	return matcher[T]{
		allow:  func(identity.Principal, Grant, T) bool { return true },
		filter: func(identity.Principal, Grant) Filter { return Filter{All: true} },
	}
}

func Site[T any](selector func(T) string) Matcher[T] {
	return matcher[T]{
		allow: func(_ identity.Principal, grant Grant, target T) bool {
			if selector == nil {
				return false
			}
			siteID := strings.TrimSpace(selector(target))
			if siteID == "" {
				return false
			}
			for _, allowed := range grant.SiteIDs {
				if allowed == siteID {
					return true
				}
			}
			return false
		},
		filter: func(_ identity.Principal, grant Grant) Filter {
			return Filter{UseSites: len(grant.SiteIDs) > 0, SiteIDs: grant.SiteIDs}
		},
	}
}

func Self[T any](selector func(T) string) Matcher[T] {
	return matcher[T]{
		allow: func(principal identity.Principal, grant Grant, target T) bool {
			if selector == nil || !grant.Self || strings.TrimSpace(principal.UserID) == "" {
				return false
			}
			return strings.TrimSpace(selector(target)) == strings.TrimSpace(principal.UserID)
		},
		filter: func(principal identity.Principal, grant Grant) Filter {
			owner := strings.TrimSpace(principal.UserID)
			return Filter{UseSelf: grant.Self && owner != "", OwnerID: owner}
		},
	}
}

func Any[T any](matchers ...Matcher[T]) Matcher[T] {
	copyOf := append([]Matcher[T](nil), matchers...)
	return matcher[T]{
		allow: func(principal identity.Principal, grant Grant, target T) bool {
			for _, current := range copyOf {
				if current != nil && current.Allows(principal, grant, target) {
					return true
				}
			}
			return false
		},
		filter: func(principal identity.Principal, grant Grant) Filter {
			combined := Filter{}
			for _, current := range copyOf {
				if current == nil {
					continue
				}
				part := current.Filter(principal, grant).Normalize()
				if part.All {
					return Filter{All: true}
				}
				if part.UseSites {
					combined.UseSites = true
					combined.SiteIDs = append(combined.SiteIDs, part.SiteIDs...)
				}
				if part.UseSelf {
					combined.UseSelf = true
					combined.OwnerID = part.OwnerID
				}
			}
			return combined.Normalize()
		},
	}
}

type Rule[T any] struct {
	Permission string
	Matcher    Matcher[T]
}

func Open[T any]() Rule[T] { return Rule[T]{} }

func Permission[T any](permission string, matcher Matcher[T]) Rule[T] {
	return Rule[T]{Permission: strings.TrimSpace(permission), Matcher: matcher}
}

func (rule Rule[T]) Authorize(ctx context.Context, resolver Resolver, principal identity.Principal, target T) error {
	if strings.TrimSpace(rule.Permission) == "" {
		return nil
	}
	if !principal.Authenticated {
		return ErrUnauthorized
	}
	if resolver == nil {
		return ErrForbidden
	}
	grant, err := resolver.Resolve(ctx, principal, rule.Permission)
	if err != nil {
		return err
	}
	if !grant.Allowed {
		return ErrForbidden
	}
	if grant.All || rule.Matcher == nil {
		return nil
	}
	if rule.Matcher.Allows(principal, grant, target) {
		return nil
	}
	return ErrForbidden
}

func (rule Rule[T]) Scope(ctx context.Context, resolver Resolver, principal identity.Principal) (Filter, error) {
	if strings.TrimSpace(rule.Permission) == "" {
		return Filter{All: true}, nil
	}
	if !principal.Authenticated {
		return Filter{}, ErrUnauthorized
	}
	if resolver == nil {
		return Filter{}, ErrForbidden
	}
	grant, err := resolver.Resolve(ctx, principal, rule.Permission)
	if err != nil {
		return Filter{}, err
	}
	if !grant.Allowed {
		return Filter{}, ErrForbidden
	}
	if grant.All || rule.Matcher == nil {
		return Filter{All: true}, nil
	}
	filter := rule.Matcher.Filter(principal, grant).Normalize()
	if filter.DenyAll() {
		return Filter{}, ErrForbidden
	}
	return filter, nil
}

func normalizeIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
