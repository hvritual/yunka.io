module github.com/hvritual/biz

go 1.25.0

toolchain go1.25.13

require (
	google.golang.org/grpc v1.82.1
	google.golang.org/protobuf v1.36.11
	gorm.io/gorm v1.25.5
	yunka.io/framework v0.0.0
)

require (
	github.com/aliyun/aliyun-log-go-sdk v0.1.127 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/go-kit/kit v0.10.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/klauspost/compress v1.19.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260807164820-c8921c73eeea // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	yunka.io/pkg v0.0.0-00010101000000-000000000000 // indirect
)

replace yunka.io/framework => ../../framework

replace yunka.io/pkg => ../../pkg

replace github.com/go-kit/kit v0.10.0 => ../../compat/go-kit-kit-log
