module yunka.io/framework

go 1.18

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/buger/jsonparser v1.0.0
	github.com/go-playground/locales v0.13.0
	github.com/go-playground/universal-translator v0.17.0
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/golang/protobuf v1.5.2
	github.com/pkg/errors v0.9.1
	github.com/valyala/fasthttp v1.16.0
	gopkg.in/go-playground/validator.v9 v9.31.0
	gorm.io/driver/mysql v1.5.2
	gorm.io/gorm v1.25.5
	yunka.io/pkg v0.0.0-00010101000000-000000000000
)

require (
	github.com/aliyun/aliyun-log-go-sdk v0.1.64 // indirect
	github.com/andybalholm/brotli v1.0.0 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/go-kit/kit v0.10.0 // indirect
	github.com/go-logfmt/logfmt v0.5.0 // indirect
	github.com/go-sql-driver/mysql v1.7.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/klauspost/compress v1.10.7 // indirect
	github.com/leodido/go-urn v1.2.0 // indirect
	github.com/onsi/ginkgo v1.14.1 // indirect
	github.com/onsi/gomega v1.10.2 // indirect
	github.com/pierrec/lz4 v2.6.0+incompatible // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	go.uber.org/atomic v1.6.0 // indirect
	golang.org/x/lint v0.0.0-20190930215403-16217165b5de // indirect
	golang.org/x/net v0.14.0 // indirect
	golang.org/x/tools v0.0.0-20210106214847-113979e3529a // indirect
	google.golang.org/protobuf v1.26.0 // indirect
	gopkg.in/go-playground/assert.v1 v1.2.1 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/yaml.v2 v2.3.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace yunka.io/pkg => ../pkg
