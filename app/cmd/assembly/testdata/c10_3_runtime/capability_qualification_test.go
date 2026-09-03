package qualification

import (
	"context"
	"net/http"
	"testing"

	devicev1 "example.com/c103qualification/contracts/device/v1"
	generatedassembly "example.com/c103qualification/internal/assembly"
	deviceapplication "example.com/c103qualification/internal/device/application"
	inventoryapplication "example.com/c103qualification/internal/inventory/application"
	siteapplication "example.com/c103qualification/internal/site/application"

	"google.golang.org/grpc"
	"github.com/hvritual/yunka.io/framework/core/modulecatalog"
	"github.com/hvritual/yunka.io/framework/operation"
	"github.com/hvritual/yunka.io/framework/platform"
)

type qualificationCache interface {
	Prefix() string
}

type qualificationCacheValue struct{ prefix string }

func (cache qualificationCacheValue) Prefix() string { return cache.prefix }

type qualificationCacheModule struct {
	key   modulecatalog.CapabilityKey[qualificationCache]
	cache qualificationCache
}

func (*qualificationCacheModule) Name() string { return "qualification-cache" }

func (module *qualificationCacheModule) ExportCapabilities() []modulecatalog.CapabilityExport {
	return []modulecatalog.CapabilityExport{modulecatalog.ExportCapability(module.key, module.cache)}
}

type capabilityDeviceQueryService struct{ cache qualificationCache }

func (service *capabilityDeviceQueryService) GetDevice(_ context.Context, request *devicev1.GetDeviceRequest) (*devicev1.DeviceDTO, error) {
	return &devicev1.DeviceDTO{Id: request.GetId(), Serial: service.cache.Prefix() + ":assembled-runtime"}, nil
}

type capabilityFactories struct {
	probe *runtimeProbe
	cache qualificationCache
}

func (factory capabilityFactories) BuildSiteManagement(generatedassembly.SiteManagementDependencies) (siteapplication.SiteApplication, error) {
	return &siteService{probe: factory.probe}, nil
}

func (factory capabilityFactories) BuildInventoryCatalog(generatedassembly.InventoryCatalogDependencies) (inventoryapplication.InventoryApplication, error) {
	return &inventoryService{probe: factory.probe}, nil
}

func (factory capabilityFactories) BuildDeviceQuery(generatedassembly.DeviceQueryDependencies) (deviceapplication.QueryApplication, error) {
	return &capabilityDeviceQueryService{cache: factory.cache}, nil
}

func (factory capabilityFactories) BuildDeviceTransfer(dependencies generatedassembly.DeviceTransferDependencies) (deviceapplication.TransferApplication, error) {
	return &deviceTransferService{site: dependencies.SiteManagement, inventory: dependencies.InventoryCatalog, probe: factory.probe}, nil
}

var _ generatedassembly.ApplicationFactories = capabilityFactories{}

func TestGeneratedAssemblyBindsExplicitTypedInfrastructureCapability(t *testing.T) {
	ctx := context.Background()
	key := modulecatalog.MustCapabilityKey[qualificationCache](
		"cache.qualification",
		"example.com/c103qualification/contracts/cache",
		"Cache",
	)
	provider, err := platform.New(platform.Options{})
	if err != nil {
		t.Fatal(err)
	}
	descriptor := modulecatalog.Descriptor{
		Name:     "qualification-cache",
		Provides: []modulecatalog.CapabilityContract{key.Contract()},
		Build: func(modulecatalog.BuildContext) (modulecatalog.Instance, error) {
			return &qualificationCacheModule{key: key, cache: qualificationCacheValue{prefix: "typed-infra"}}, nil
		},
	}
	executor := operation.NewExecutor(nil)
	result, err := generatedassembly.Bootstrap(ctx, generatedassembly.BootstrapOptions{
		Platform:          provider,
		AdditionalModules: []modulecatalog.Descriptor{descriptor},
		BindRuntimeWithCapabilities: func(_ context.Context, _ *platform.Provider, capabilities modulecatalog.CapabilitySet) (generatedassembly.RuntimeBindings, error) {
			cache, err := modulecatalog.ResolveCapability(capabilities, key)
			if err != nil {
				return generatedassembly.RuntimeBindings{}, err
			}
			return generatedassembly.RuntimeBindings{
				Factories: capabilityFactories{probe: newRuntimeProbe(), cache: cache},
				Executor:  executor,
			}, nil
		},
		Transports: generatedassembly.TransportBindings{HTTP: http.NewServeMux(), RPC: grpc.NewServer()},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.App.Shutdown(context.Background())

	response, err := result.Applications.DeviceQuery.GetDevice(ctx, &devicev1.GetDeviceRequest{Id: "device-typed"})
	if err != nil {
		t.Fatal(err)
	}
	if response.GetSerial() != "typed-infra:assembled-runtime" {
		t.Fatalf("typed capability was not injected into Application factory: serial=%q", response.GetSerial())
	}
	if got := len(result.App.Diagnostics(ctx).Modules); got != 4 {
		t.Fatalf("runtime composed module count=%d want=4", got)
	}
}
