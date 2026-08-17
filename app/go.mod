module yunka.io/framework

go 1.21

toolchain go1.21.2

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/buger/jsonparser v1.0.0
	github.com/go-playground/locales v0.13.0
	github.com/go-playground/universal-translator v0.17.0
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/golang/protobuf v1.5.3
	github.com/kataras/golog v0.1.5
	github.com/pkg/errors v0.9.1
	github.com/urfave/cli v1.22.5
	github.com/valyala/fasthttp v1.16.0
	gopkg.in/go-playground/validator.v9 v9.31.0
	gorm.io/driver/mysql v1.5.2
	gorm.io/gorm v1.25.5
	yunka.io/pkg v0.0.0-00010101000000-000000000000
)

require (
	github.com/leodido/go-urn v1.2.0 // indirect
	gopkg.in/go-playground/assert.v1 v1.2.1 // indirect
)

replace yunka.io/pkg => ../pkg
