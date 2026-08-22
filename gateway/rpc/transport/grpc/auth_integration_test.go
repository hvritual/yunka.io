package grpc

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"

	stdgrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpccredentials "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	empty "google.golang.org/protobuf/types/known/emptypb"
	"yunka.io/framework/core/identity"
	coremiddleware "yunka.io/framework/core/middleware"
)

const testIdentityMethod = "/c1.Identity/WhoAmI"

type identityTestService interface {
	WhoAmI(context.Context, *empty.Empty) (*empty.Empty, error)
}

type identityTestServer struct {
	seen chan identity.Principal
}

func (server *identityTestServer) WhoAmI(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	principal, ok := identity.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "principal missing")
	}
	select {
	case server.seen <- principal:
	default:
	}
	return &empty.Empty{}, nil
}

var identityTestServiceDesc = stdgrpc.ServiceDesc{
	ServiceName: "c1.Identity",
	HandlerType: (*identityTestService)(nil),
	Methods: []stdgrpc.MethodDesc{{
		MethodName: "WhoAmI",
		Handler:    identityTestWhoAmIHandler,
	}},
}

func identityTestWhoAmIHandler(service interface{}, ctx context.Context, decode func(interface{}) error, interceptor stdgrpc.UnaryServerInterceptor) (interface{}, error) {
	request := new(empty.Empty)
	if err := decode(request); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return service.(identityTestService).WhoAmI(ctx, request)
	}
	info := &stdgrpc.UnaryServerInfo{Server: service, FullMethod: testIdentityMethod}
	handler := func(child context.Context, value interface{}) (interface{}, error) {
		return service.(identityTestService).WhoAmI(child, value.(*empty.Empty))
	}
	return interceptor(ctx, request, info, handler)
}

func TestServiceCredentialsOverTLSBufconn(t *testing.T) {
	serverTLS, clientTLS := testTLSCredentials(t)
	verifier, err := NewStaticServiceTokenVerifier([]StaticServiceCredential{{
		Token: testServiceTokenA,
		Principal: identity.Principal{
			Subject: "gateway", Roles: []string{"rpc-caller"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}

	listener := bufconn.Listen(1 << 20)
	service := &identityTestServer{seen: make(chan identity.Principal, 1)}
	grpcServer := stdgrpc.NewServer(
		stdgrpc.Creds(serverTLS),
		stdgrpc.UnaryInterceptor(AuthenticatedUnaryServerInterceptor(coremiddleware.New(), verifier)),
	)
	grpcServer.RegisterService(&identityTestServiceDesc, service)
	serveResult := make(chan error, 1)
	go func() { serveResult <- grpcServer.Serve(listener) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = listener.Close()
		select {
		case <-serveResult:
		default:
		}
	})

	dialer := func(context.Context, string) (net.Conn, error) { return listener.Dial() }
	validCredentials, err := NewStaticServiceTokenCredentials(testServiceTokenA)
	if err != nil {
		t.Fatal(err)
	}
	validConnection := dialTestConnection(t, dialer, clientTLS, validCredentials)
	var response empty.Empty
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := validConnection.Invoke(ctx, testIdentityMethod, &empty.Empty{}, &response); err != nil {
		t.Fatal(err)
	}
	select {
	case principal := <-service.seen:
		if principal.Subject != "gateway" || !principal.Authenticated || principal.AuthMethod != identity.AuthMethodServiceToken || !principal.HasRole("rpc-caller") {
			t.Fatalf("principal=%+v", principal)
		}
	case <-ctx.Done():
		t.Fatal("service did not observe the authenticated principal")
	}

	invalidCredentials, err := NewStaticServiceTokenCredentials(testServiceTokenB)
	if err != nil {
		t.Fatal(err)
	}
	invalidConnection := dialTestConnection(t, dialer, clientTLS, invalidCredentials)
	if err := invalidConnection.Invoke(ctx, testIdentityMethod, &empty.Empty{}, &empty.Empty{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("invalid token err=%v", err)
	}

	missingConnection := dialTestConnection(t, dialer, clientTLS, nil)
	if err := missingConnection.Invoke(ctx, testIdentityMethod, &empty.Empty{}, &empty.Empty{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing token err=%v", err)
	}
}

func dialTestConnection(t *testing.T, dialer func(context.Context, string) (net.Conn, error), transport grpccredentials.TransportCredentials, perRPC grpccredentials.PerRPCCredentials) *stdgrpc.ClientConn {
	t.Helper()
	options := []stdgrpc.DialOption{
		stdgrpc.WithContextDialer(dialer),
		stdgrpc.WithTransportCredentials(transport),
		stdgrpc.WithBlock(),
	}
	if perRPC != nil {
		options = append(options, stdgrpc.WithPerRPCCredentials(perRPC))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := stdgrpc.DialContext(ctx, "passthrough:///bufconn.test", options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	return connection
}

func testTLSCredentials(t *testing.T) (grpccredentials.TransportCredentials, grpccredentials.TransportCredentials) {
	t.Helper()
	caPublicKey, caPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "yunka C1 test CA"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, caPublicKey, caPrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	caCertificate, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}

	serverPublicKey, serverPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "bufconn.test"},
		DNSNames:     []string{"bufconn.test"},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCertificate, serverPublicKey, caPrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	certificate := tls.Certificate{
		Certificate: [][]byte{serverDER, caDER},
		PrivateKey:  serverPrivateKey,
	}
	roots := x509.NewCertPool()
	roots.AddCert(caCertificate)
	server := grpccredentials.NewServerTLSFromCert(&certificate)
	client := grpccredentials.NewClientTLSFromCert(roots, "bufconn.test")
	return server, client
}
