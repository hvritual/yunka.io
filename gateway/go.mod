module yunka.io/gateway

go 1.14

require (
	github.com/aliyun/aliyun-log-go-sdk v0.1.64
	github.com/buger/jsonparser v1.0.0
	github.com/didi/gendry v1.6.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.4.3
	github.com/google/uuid v1.1.2
	github.com/panjf2000/ants v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.9.0
	github.com/valyala/fasthttp v1.16.0
	google.golang.org/grpc v1.33.1
	gorm.io/driver/sqlite v1.1.4
	gorm.io/gorm v1.20.8
	yunka.io/framework v0.0.0-00010101000000-000000000000
	yunka.io/pkg v0.0.0-00010101000000-000000000000
)

replace yunka.io/pkg => ../pkg

replace yunka.io/framework => ../framework
