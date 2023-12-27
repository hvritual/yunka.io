module yunka.io/pkg

go 1.18

replace github.com/coreos/bbolt v1.3.3 => go.etcd.io/bbolt v1.3.3

replace google.golang.org/grpc => google.golang.org/grpc v1.26.0

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/DATA-DOG/go-sqlmock v1.5.1
	github.com/aliyun/aliyun-log-go-sdk v0.1.64
	github.com/buger/jsonparser v1.0.0
	github.com/coreos/bbolt v1.3.3 // indirect
	github.com/coreos/etcd v3.3.21+incompatible
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/go-kit/kit v0.10.0
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/go-redis/redismock/v9 v9.2.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/protobuf v1.5.2
	github.com/google/uuid v1.1.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.2 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/jonboulle/clockwork v0.2.2 // indirect
	github.com/micro/mdns v0.3.0
	github.com/mitchellh/hashstructure v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.9.0 // indirect
	github.com/rs/xid v1.2.1
	github.com/stretchr/testify v1.8.0
	github.com/tmc/grpc-websocket-proxy v0.0.0-20200427203606-3cfed13b9966 // indirect
	github.com/xuri/excelize/v2 v2.8.0
	go.etcd.io/bbolt v1.3.5 // indirect
	go.uber.org/zap v1.16.0 // indirect
	golang.org/x/crypto v0.12.0
	golang.org/x/time v0.0.0-20201208040808-7e3f01d25324 // indirect
	google.golang.org/genproto v0.0.0-20201214200347-8c77b98c765d // indirect
	google.golang.org/grpc v1.33.1
	gorm.io/driver/mysql v1.5.2
	gorm.io/gorm v1.25.5
	sigs.k8s.io/yaml v1.2.0 // indirect

)
