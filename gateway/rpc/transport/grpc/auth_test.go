package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	stdgrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpccredentials "google.golang.org/grpc/credentials"
	grpcmetadata "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"github.com/hvritual/yunka.io/framework/core/identity"
	coremiddleware "github.com/hvritual/yunka.io/framework/core/middleware"
)

const (
	testServiceTokenA = "service-token-a-0123456789abcdef0123456789abcdef"
	testServiceTokenB = "service-token-b-0123456789abcdef0123456789abcdef"
)

type testAuthInfo struct {
	grpccredentials.CommonAuthInfo
}

func (testAuthInfo) AuthType() string { return "test-secure-transport" }

func secureIncomingContext(token string) context.Context {
	ctx := context.Background()
	if token != "" {
		ctx = grpcmetadata.NewIncomingContext(ctx, grpcmetadata.Pairs(ServiceAuthorizationMetadata, "Bearer "+token))
	}
	return peer.NewContext(ctx, &peer.Peer{AuthInfo: testAuthInfo{CommonAuthInfo: grpccredentials.CommonAuthInfo{SecurityLevel: grpccredentials.PrivacyAndIntegrity}}})
}

func TestStaticServiceTokenVerifierValidatesAndNormalizesPrincipal(t *testing.T) {
	verifier, err := NewStaticServiceTokenVerifier([]StaticServiceCredential{{
		Token: testServiceTokenA,
		Principal: identity.Principal{
			Subject: " gateway ", Roles: []string{"caller", "caller", " "},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	principal, err := verifier.Verify(secureIncomingContext(testServiceTokenA))
	if err != nil {
		t.Fatal(err)
	}
	if !principal.Authenticated || principal.Subject != "gateway" || principal.AuthMethod != identity.AuthMethodServiceToken {
		t.Fatalf("principal=%+v", principal)
	}
	if len(principal.Roles) != 1 || principal.Roles[0] != "caller" {
		t.Fatalf("roles=%v", principal.Roles)
	}
}

func TestStaticServiceTokenVerifierRejectsWeakDuplicateAndInsecureCredentials(t *testing.T) {
	if _, err := NewStaticServiceTokenVerifier([]StaticServiceCredential{{Token: "short", Principal: identity.Principal{Subject: "gateway"}}}); err == nil {
		t.Fatal("weak token accepted")
	}
	if _, err := NewStaticServiceTokenVerifier([]StaticServiceCredential{
		{Token: testServiceTokenA, Principal: identity.Principal{Subject: "gateway-a"}},
		{Token: testServiceTokenA, Principal: identity.Principal{Subject: "gateway-b"}},
	}); err == nil {
		t.Fatal("duplicate token accepted")
	}
	if _, err := NewStaticServiceTokenVerifier([]StaticServiceCredential{{
		Token: testServiceTokenA, Principal: identity.Principal{Subject: "gateway", Roles: []string{"admin\nroot"}},
	}}); err == nil {
		t.Fatal("control character in role accepted")
	}
	verifier, err := NewStaticServiceTokenVerifier([]StaticServiceCredential{{Token: testServiceTokenA, Principal: identity.Principal{Subject: "gateway"}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := grpcmetadata.NewIncomingContext(context.Background(), grpcmetadata.Pairs(ServiceAuthorizationMetadata, "Bearer "+testServiceTokenA))
	if _, err := verifier.Verify(ctx); !errors.Is(err, ErrServiceCredentialInsecureTransport) {
		t.Fatalf("err=%v", err)
	}
}

func TestStaticServiceTokenVerifierSupportsRotation(t *testing.T) {
	verifier, err := NewStaticServiceTokenVerifier([]StaticServiceCredential{
		{Token: testServiceTokenA, Principal: identity.Principal{Subject: "gateway-old"}},
		{Token: testServiceTokenB, Principal: identity.Principal{Subject: "gateway-new"}},
	}, AllowInsecureServiceCredentialsForDevelopment())
	if err != nil {
		t.Fatal(err)
	}
	for token, want := range map[string]string{testServiceTokenA: "gateway-old", testServiceTokenB: "gateway-new"} {
		ctx := grpcmetadata.NewIncomingContext(context.Background(), grpcmetadata.Pairs(ServiceAuthorizationMetadata, "Bearer "+token))
		principal, err := verifier.Verify(ctx)
		if err != nil || principal.Subject != want {
			t.Fatalf("token subject=%q err=%v want=%q", principal.Subject, err, want)
		}
	}
}

func TestAuthenticatedUnaryServerInterceptorContainsVerifierPanic(t *testing.T) {
	interceptor := AuthenticatedUnaryServerInterceptor(coremiddleware.New(), CredentialVerifierFunc(func(context.Context) (identity.Principal, error) {
		panic("credential backend secret")
	}))
	_, err := interceptor(context.Background(), "request", &stdgrpc.UnaryServerInfo{FullMethod: "/test.Service/Get"}, func(context.Context, interface{}) (interface{}, error) {
		return "response", nil
	})
	if status.Code(err) != codes.Unauthenticated || strings.Contains(status.Convert(err).Message(), "secret") {
		t.Fatalf("err=%v", err)
	}
}

func TestAuthenticatedUnaryServerInterceptorFailsClosed(t *testing.T) {
	called := false
	interceptor := AuthenticatedUnaryServerInterceptor(coremiddleware.New(), CredentialVerifierFunc(func(context.Context) (identity.Principal, error) {
		return identity.Principal{}, errors.New("secret verifier detail")
	}))
	_, err := interceptor(context.Background(), "request", &stdgrpc.UnaryServerInfo{FullMethod: "/test.Service/Get"}, func(context.Context, interface{}) (interface{}, error) {
		called = true
		return "response", nil
	})
	if called {
		t.Fatal("handler called after authentication failure")
	}
	if status.Code(err) != codes.Unauthenticated || strings.Contains(status.Convert(err).Message(), "secret verifier detail") {
		t.Fatalf("err=%v", err)
	}
}

func TestAuthenticatedUnaryServerInterceptorSetsPrincipalBeforeMiddleware(t *testing.T) {
	var middlewarePrincipal identity.Principal
	chain := coremiddleware.New(func(next coremiddleware.Handler) coremiddleware.Handler {
		return func(ctx context.Context) error {
			middlewarePrincipal, _ = identity.FromContext(ctx)
			return next(ctx)
		}
	})
	verifier := CredentialVerifierFunc(func(context.Context) (identity.Principal, error) {
		return identity.Principal{Subject: "gateway", AuthMethod: identity.AuthMethodServiceToken, Authenticated: true}, nil
	})
	interceptor := AuthenticatedUnaryServerInterceptor(chain, verifier)
	_, err := interceptor(context.Background(), "request", &stdgrpc.UnaryServerInfo{FullMethod: "/test.Service/Get"}, func(ctx context.Context, _ interface{}) (interface{}, error) {
		principal, ok := identity.FromContext(ctx)
		if !ok || principal.Subject != "gateway" {
			t.Fatalf("handler principal=%+v ok=%v", principal, ok)
		}
		return "response", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if middlewarePrincipal.Subject != "gateway" || !middlewarePrincipal.Authenticated {
		t.Fatalf("middleware principal=%+v", middlewarePrincipal)
	}
}

func TestCompatibilityUnaryInterceptorDoesNotTrustIdentityMetadata(t *testing.T) {
	ctx := grpcmetadata.NewIncomingContext(context.Background(), grpcmetadata.Pairs(
		"x-yunka-subject", "spoofed",
		"x-yunka-role", "admin",
		ServiceAuthorizationMetadata, "Bearer "+testServiceTokenA,
	))
	interceptor := UnaryServerInterceptor(coremiddleware.New())
	_, err := interceptor(ctx, "request", &stdgrpc.UnaryServerInfo{FullMethod: "/test.Service/Get"}, func(child context.Context, _ interface{}) (interface{}, error) {
		if principal, ok := identity.FromContext(child); ok || principal.Authenticated {
			t.Fatalf("spoofed principal trusted: %+v ok=%v", principal, ok)
		}
		return "response", nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestStaticServiceTokenCredentialsRequireTransportSecurity(t *testing.T) {
	credentials, err := NewStaticServiceTokenCredentials(testServiceTokenA)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !credentials.RequireTransportSecurity() || metadata[ServiceAuthorizationMetadata] != "Bearer "+testServiceTokenA {
		t.Fatalf("metadata=%v secure=%v", metadata, credentials.RequireTransportSecurity())
	}
}

type testServerStream struct {
	stdgrpc.ServerStream
	ctx context.Context
}

func (stream *testServerStream) Context() context.Context { return stream.ctx }

func TestAuthenticatedStreamServerInterceptorSetsPrincipal(t *testing.T) {
	verifier := CredentialVerifierFunc(func(context.Context) (identity.Principal, error) {
		return identity.Principal{Subject: "stream-client", AuthMethod: identity.AuthMethodServiceToken, Authenticated: true}, nil
	})
	interceptor := AuthenticatedStreamServerInterceptor(coremiddleware.New(), verifier)
	err := interceptor(nil, &testServerStream{ctx: context.Background()}, &stdgrpc.StreamServerInfo{FullMethod: "/test.Service/Watch"}, func(_ interface{}, stream stdgrpc.ServerStream) error {
		principal, ok := identity.FromContext(stream.Context())
		if !ok || principal.Subject != "stream-client" {
			t.Fatalf("principal=%+v ok=%v", principal, ok)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
