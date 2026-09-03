package qualification

import (
	"context"
	"testing"

	cachecontract "example.com/c103qualification/contracts/cache"

	"gorm.io/gorm"
	"github.com/hvritual/yunka.io/framework/core/modulecatalog"
	"github.com/hvritual/yunka.io/framework/platform"
)

func qualificationCapabilityDescriptors(prefix string) []modulecatalog.Descriptor {
	key := modulecatalog.MustCapabilityKey[cachecontract.Cache](
		"cache.qualification",
		"example.com/c103qualification/contracts/cache",
		"Cache",
	)
	return []modulecatalog.Descriptor{{
		Name:     "qualification-cache",
		Provides: []modulecatalog.CapabilityContract{key.Contract()},
		Build: func(modulecatalog.BuildContext) (modulecatalog.Instance, error) {
			return &qualificationCacheModule{key: key, cache: qualificationCacheValue{prefix: prefix}}, nil
		},
	}}
}

func qualificationCapabilityPlatform(t *testing.T) *platform.Provider {
	t.Helper()
	provider, err := platform.New(platform.Options{Databases: map[string]platform.DatabaseFactory{
		"primary": platform.DatabaseFactoryFunc(func(context.Context, string) (platform.DatabaseResource, error) {
			return platform.DatabaseResource{
				Database:     &gorm.DB{},
				HealthFunc:   func(context.Context) error { return nil },
				ShutdownFunc: func(context.Context) error { return nil },
			}, nil
		}),
	}})
	if err != nil {
		t.Fatal(err)
	}
	return provider
}
