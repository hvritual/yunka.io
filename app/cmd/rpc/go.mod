module yunka-rpc

go 1.25.0

toolchain go1.25.13

replace google.golang.org/genproto => google.golang.org/genproto v0.0.0-20260414002931-afd174a4e478

require (
	github.com/golang/protobuf v1.5.4
	github.com/pkg/errors v0.9.1
	github.com/siddontang/go v0.0.0-20180604090527-bdc77568d726
	google.golang.org/genproto/googleapis/api v0.0.0-20260810153831-ec0a7760b754
)

require google.golang.org/protobuf v1.36.11 // indirect
