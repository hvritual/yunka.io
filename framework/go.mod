module yunka.io/framework

go 1.14

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/buger/jsonparser v1.0.0
	github.com/go-playground/locales v0.13.0
	github.com/go-playground/universal-translator v0.17.0
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/golang/protobuf v1.4.3
	github.com/leodido/go-urn v1.2.0 // indirect
	github.com/onsi/ginkgo v1.14.1 // indirect
	github.com/onsi/gomega v1.10.2 // indirect
	github.com/pkg/errors v0.9.1
	github.com/valyala/fasthttp v1.16.0
	gopkg.in/go-playground/assert.v1 v1.2.1 // indirect
	gopkg.in/go-playground/validator.v9 v9.31.0
	gorm.io/driver/mysql v1.0.3
	gorm.io/gorm v1.20.8
	yunka.io/pkg v0.0.0-00010101000000-000000000000
)

replace yunka.io/pkg => ../pkg
