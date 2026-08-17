module yunka.io/app

go 1.25.0

toolchain go1.25.13

replace google.golang.org/genproto => google.golang.org/genproto v0.0.0-20260414002931-afd174a4e478

require (
	github.com/kataras/golog v0.1.5
	github.com/pkg/errors v0.9.1
	github.com/urfave/cli v1.22.5
	yunka.io/pkg v0.0.0-00010101000000-000000000000
)

require (
	github.com/buger/jsonparser v1.6.1 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.0-20190314233015-f79a8a8ca69d // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/kataras/pio v0.0.10 // indirect
	github.com/rs/xid v1.2.1 // indirect
	github.com/russross/blackfriday/v2 v2.0.1 // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

replace yunka.io/pkg => ../pkg
