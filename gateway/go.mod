module yunka.io/gateway

go 1.18

require (
	github.com/aliyun/aliyun-log-go-sdk v0.1.64
	github.com/buger/jsonparser v1.0.0
	github.com/didi/gendry v1.6.0
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

require (
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/andybalholm/brotli v1.0.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/go-kit/kit v0.10.0 // indirect
	github.com/go-logfmt/logfmt v0.5.0 // indirect
	github.com/go-playground/locales v0.13.0 // indirect
	github.com/go-playground/universal-translator v0.17.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.1 // indirect
	github.com/klauspost/compress v1.10.7 // indirect
	github.com/leodido/go-urn v1.2.0 // indirect
	github.com/mattn/go-sqlite3 v1.14.5 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/pierrec/lz4 v2.6.0+incompatible // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.15.0 // indirect
	github.com/prometheus/procfs v0.2.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	go.uber.org/atomic v1.5.0 // indirect
	golang.org/x/crypto v0.12.0 // indirect
	golang.org/x/lint v0.0.0-20190930215403-16217165b5de // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/sys v0.11.0 // indirect
	golang.org/x/text v0.12.0 // indirect
	golang.org/x/tools v0.6.0 // indirect
	google.golang.org/genproto v0.0.0-20201214200347-8c77b98c765d // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	gopkg.in/go-playground/validator.v9 v9.31.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace yunka.io/pkg => ../pkg

replace yunka.io/framework => ../framework
