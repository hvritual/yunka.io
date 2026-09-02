package grpc

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"

	grpccredentials "google.golang.org/grpc/credentials"
	grpcmetadata "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"github.com/hvritual/yunka.io/framework/core/identity"
)

const (
	// ServiceAuthorizationMetadata is intentionally distinct from the end-user
	// Authorization header. Gateways must not turn a forwarded user token into a
	// trusted service Principal at an RPC boundary.
	ServiceAuthorizationMetadata = "x-yunka-service-authorization"

	MinServiceTokenBytes    = 32
	MaxServiceTokenBytes    = 4096
	MaxServiceIdentityBytes = 256
	MaxServiceRoleBytes     = 128
	MaxServiceRoles         = 64
)

var (
	ErrServiceCredentialMissing           = errors.New("grpc: service credential is missing")
	ErrServiceCredentialInvalid           = errors.New("grpc: service credential is invalid")
	ErrServiceCredentialInsecureTransport = errors.New("grpc: service credential requires privacy and integrity")
)

// CredentialVerifier establishes a trusted Principal from server-side
// credential validation. Implementations must never treat caller-supplied
// tenant, user, or role metadata as proof of identity.
type CredentialVerifier interface {
	Verify(context.Context) (identity.Principal, error)
}

type CredentialVerifierFunc func(context.Context) (identity.Principal, error)

func (verify CredentialVerifierFunc) Verify(ctx context.Context) (identity.Principal, error) {
	if verify == nil {
		return identity.Principal{}, ErrServiceCredentialInvalid
	}
	return verify(ctx)
}

// StaticServiceCredential binds a secret token to one server-established
// Principal. Multiple entries allow overlap during token rotation.
type StaticServiceCredential struct {
	Token     string
	Principal identity.Principal
}

type staticServiceCredential struct {
	digest    [sha256.Size]byte
	principal identity.Principal
}

type StaticServiceTokenVerifier struct {
	credentials   []staticServiceCredential
	allowInsecure bool
}

type StaticServiceTokenVerifierOption func(*StaticServiceTokenVerifier) error

// AllowInsecureServiceCredentialsForDevelopment disables transport-security
// enforcement. It exists only for loopback tests and local development; a
// production deployment should use TLS, mTLS, ALTS, or an equivalent channel
// that provides privacy and integrity.
func AllowInsecureServiceCredentialsForDevelopment() StaticServiceTokenVerifierOption {
	return func(verifier *StaticServiceTokenVerifier) error {
		verifier.allowInsecure = true
		return nil
	}
}

func NewStaticServiceTokenVerifier(credentials []StaticServiceCredential, options ...StaticServiceTokenVerifierOption) (*StaticServiceTokenVerifier, error) {
	if len(credentials) == 0 {
		return nil, errors.New("grpc: at least one service credential is required")
	}
	verifier := &StaticServiceTokenVerifier{}
	for _, option := range options {
		if option != nil {
			if err := option(verifier); err != nil {
				return nil, err
			}
		}
	}

	seen := make(map[[sha256.Size]byte]struct{}, len(credentials))
	verifier.credentials = make([]staticServiceCredential, 0, len(credentials))
	for index, credential := range credentials {
		token, err := normalizeServiceToken(credential.Token)
		if err != nil {
			return nil, fmt.Errorf("grpc: service credential %d: %w", index, err)
		}
		principal, err := normalizeServicePrincipal(credential.Principal)
		if err != nil {
			return nil, fmt.Errorf("grpc: service credential %d: %w", index, err)
		}
		digest := sha256.Sum256([]byte(token))
		if _, duplicate := seen[digest]; duplicate {
			return nil, fmt.Errorf("grpc: service credential %d duplicates an existing token", index)
		}
		seen[digest] = struct{}{}
		verifier.credentials = append(verifier.credentials, staticServiceCredential{digest: digest, principal: principal})
	}
	return verifier, nil
}

func verifyCredential(ctx context.Context, verifier CredentialVerifier) (principal identity.Principal, err error) {
	defer func() {
		if recover() != nil {
			principal = identity.Principal{}
			err = ErrServiceCredentialInvalid
		}
	}()
	return verifier.Verify(ctx)
}

func (verifier *StaticServiceTokenVerifier) Verify(ctx context.Context) (identity.Principal, error) {
	if verifier == nil || len(verifier.credentials) == 0 {
		return identity.Principal{}, ErrServiceCredentialInvalid
	}
	if !verifier.allowInsecure && !transportProvidesPrivacyAndIntegrity(ctx) {
		return identity.Principal{}, ErrServiceCredentialInsecureTransport
	}
	metadata, ok := grpcmetadata.FromIncomingContext(ctx)
	if !ok {
		return identity.Principal{}, ErrServiceCredentialMissing
	}
	values := metadata.Get(ServiceAuthorizationMetadata)
	if len(values) == 0 {
		return identity.Principal{}, ErrServiceCredentialMissing
	}
	if len(values) != 1 {
		return identity.Principal{}, ErrServiceCredentialInvalid
	}
	token, err := parseBearerCredential(values[0])
	if err != nil {
		return identity.Principal{}, ErrServiceCredentialInvalid
	}
	candidate := sha256.Sum256([]byte(token))
	match := -1
	for index := range verifier.credentials {
		if subtle.ConstantTimeCompare(candidate[:], verifier.credentials[index].digest[:]) == 1 {
			match = index
		}
	}
	if match < 0 {
		return identity.Principal{}, ErrServiceCredentialInvalid
	}
	return verifier.credentials[match].principal.Clone(), nil
}

// StaticServiceTokenCredentials implements gRPC PerRPCCredentials and requires
// transport security. It never exposes the token through runtime diagnostics.
type StaticServiceTokenCredentials struct {
	token string
}

func NewStaticServiceTokenCredentials(token string) (*StaticServiceTokenCredentials, error) {
	normalized, err := normalizeServiceToken(token)
	if err != nil {
		return nil, err
	}
	return &StaticServiceTokenCredentials{token: normalized}, nil
}

func (credentials *StaticServiceTokenCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	if credentials == nil || credentials.token == "" {
		return nil, ErrServiceCredentialInvalid
	}
	return map[string]string{ServiceAuthorizationMetadata: "Bearer " + credentials.token}, nil
}

func (*StaticServiceTokenCredentials) RequireTransportSecurity() bool { return true }

func normalizeServiceToken(token string) (string, error) {
	if len(token) < MinServiceTokenBytes {
		return "", fmt.Errorf("service token must contain at least %d bytes", MinServiceTokenBytes)
	}
	if len(token) > MaxServiceTokenBytes {
		return "", fmt.Errorf("service token must not exceed %d bytes", MaxServiceTokenBytes)
	}
	if strings.TrimSpace(token) != token {
		return "", errors.New("service token must not have surrounding whitespace")
	}
	for index := 0; index < len(token); index++ {
		if token[index] < 0x21 || token[index] > 0x7e {
			return "", errors.New("service token must contain visible ASCII characters only")
		}
	}
	return token, nil
}

func normalizeServicePrincipal(principal identity.Principal) (identity.Principal, error) {
	principal = principal.Clone()
	var err error
	principal.Subject, err = normalizeIdentityValue(principal.Subject, "subject", true, MaxServiceIdentityBytes)
	if err != nil {
		return identity.Principal{}, err
	}
	principal.TenantID, err = normalizeIdentityValue(principal.TenantID, "tenant ID", false, MaxServiceIdentityBytes)
	if err != nil {
		return identity.Principal{}, err
	}
	principal.UserID, err = normalizeIdentityValue(principal.UserID, "user ID", false, MaxServiceIdentityBytes)
	if err != nil {
		return identity.Principal{}, err
	}
	if len(principal.Roles) > MaxServiceRoles {
		return identity.Principal{}, fmt.Errorf("service principal must not contain more than %d roles", MaxServiceRoles)
	}
	roles := make([]string, 0, len(principal.Roles))
	seen := make(map[string]struct{}, len(principal.Roles))
	for _, role := range principal.Roles {
		role, err = normalizeIdentityValue(role, "role", false, MaxServiceRoleBytes)
		if err != nil {
			return identity.Principal{}, err
		}
		if role == "" {
			continue
		}
		if _, duplicate := seen[role]; duplicate {
			continue
		}
		seen[role] = struct{}{}
		roles = append(roles, role)
	}
	principal.Roles = roles
	principal.AuthMethod = identity.AuthMethodServiceToken
	principal.Authenticated = true
	return principal, nil
}

func normalizeIdentityValue(value, field string, required bool, maximum int) (string, error) {
	value = strings.TrimSpace(value)
	if required && value == "" {
		return "", fmt.Errorf("service principal %s is required", field)
	}
	if len(value) > maximum {
		return "", fmt.Errorf("service principal %s must not exceed %d bytes", field, maximum)
	}
	if strings.IndexFunc(value, func(current rune) bool { return current < 0x20 || current == 0x7f }) >= 0 {
		return "", fmt.Errorf("service principal %s contains control characters", field)
	}
	return value, nil
}

func parseBearerCredential(header string) (string, error) {
	parts := strings.Fields(strings.TrimSpace(header))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", ErrServiceCredentialInvalid
	}
	return normalizeServiceToken(parts[1])
}

func transportProvidesPrivacyAndIntegrity(ctx context.Context) bool {
	current, ok := peer.FromContext(ctx)
	if !ok || current.AuthInfo == nil {
		return false
	}
	type commonAuthInfoProvider interface {
		GetCommonAuthInfo() grpccredentials.CommonAuthInfo
	}
	provider, ok := current.AuthInfo.(commonAuthInfoProvider)
	if !ok {
		return false
	}
	return provider.GetCommonAuthInfo().SecurityLevel >= grpccredentials.PrivacyAndIntegrity
}
