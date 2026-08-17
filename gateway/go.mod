module yunka.io/gateway

go 1.25.0

toolchain go1.25.13

replace google.golang.org/genproto => google.golang.org/genproto v0.0.0-20260414002931-afd174a4e478

require (
	github.com/aliyun/aliyun-log-go-sdk v0.1.64
	github.com/buger/jsonparser v1.6.1
	github.com/didi/gendry v1.6.0
	github.com/golang/protobuf v1.5.4
	github.com/google/uuid v1.6.0
	github.com/panjf2000/ants v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.21.1
	github.com/valyala/fasthttp v1.73.0
	google.golang.org/grpc v1.82.1
	gorm.io/driver/sqlite v1.5.2
	gorm.io/gorm v1.25.5
	yunka.io/framework v0.0.0-00010101000000-000000000000
	yunka.io/pkg v0.0.0-00010101000000-000000000000
)

require (
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/andybalholm/brotli v1.2.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-kit/kit v0.10.0 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-playground/locales v0.13.0 // indirect
	github.com/go-playground/universal-translator v0.17.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/klauspost/compress v1.19.1 // indirect
	github.com/leodido/go-urn v1.2.0 // indirect
	github.com/mattn/go-sqlite3 v1.14.17 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pierrec/lz4 v2.6.0+incompatible // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.63.0 // indirect
	github.com/prometheus/procfs v0.16.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260807164820-c8921c73eeea // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/go-playground/validator.v9 v9.31.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
)

replace yunka.io/pkg => ../pkg

replace yunka.io/framework => ../framework
