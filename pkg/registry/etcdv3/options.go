package etcdv3

import (
	"github.com/hvritual/yunka.io/pkg/registry"
	"context"
)

/**
* @Description: TODO
* @date 2019-07-16
* @version V1.0
 */
type authKey struct{}

type authCreds struct {
	Username string
	Password string
}

// Auth allows you to specify username/password
func Auth(username, password string) registry.Option {
	return func(o *registry.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, authKey{}, &authCreds{Username: username, Password: password})
	}
}
