package modulecatalog

import (
	"strings"
	"testing"
)

type testCache interface {
	Get(string) string
}

type testCacheValue struct{ prefix string }

func (cache testCacheValue) Get(key string) string { return cache.prefix + key }

type capabilityTestModule struct {
	name    string
	exports []CapabilityExport
}

func (module capabilityTestModule) Name() string { return module.name }
func (module capabilityTestModule) ExportCapabilities() []CapabilityExport {
	return append([]CapabilityExport(nil), module.exports...)
}

func capabilityDescriptor(name string, key CapabilityKey[testCache]) Descriptor {
	return Descriptor{
		Name:     name,
		Provides: []CapabilityContract{key.Contract()},
		Build:    func(BuildContext) (Instance, error) { return nil, nil },
	}
}

func TestCapabilityExportResolveIsTyped(t *testing.T) {
	key := MustCapabilityKey[testCache]("cache.default", "example.com/contracts/cache", "Cache")
	module := capabilityTestModule{name: "redis-cache", exports: []CapabilityExport{ExportCapability(key, testCacheValue{prefix: "redis:"})}}
	set, err := CollectCapabilities([]Descriptor{capabilityDescriptor("redis-cache", key)}, []Instance{module})
	if err != nil {
		t.Fatal(err)
	}
	cache, err := ResolveCapability(set, key)
	if err != nil {
		t.Fatal(err)
	}
	if got := cache.Get("device:1"); got != "redis:device:1" {
		t.Fatalf("cache.Get()=%q", got)
	}
	if set.Len() != 1 || len(set.Names()) != 1 || set.Names()[0] != "cache.default" {
		t.Fatalf("unexpected capability set: len=%d names=%v", set.Len(), set.Names())
	}
}

func TestCatalogSealFailsClosedOnDuplicateProvider(t *testing.T) {
	key := MustCapabilityKey[testCache]("cache.default", "example.com/contracts/cache", "Cache")
	catalog := New()
	catalog.MustRegister(capabilityDescriptor("redis-a", key))
	catalog.MustRegister(capabilityDescriptor("redis-b", key))
	if _, err := catalog.Seal(); err == nil || !strings.Contains(err.Error(), "multiple providers") {
		t.Fatalf("duplicate provider error=%v", err)
	}
}

func TestCatalogDuplicateProviderDiagnosticIsRegistrationOrderIndependent(t *testing.T) {
	key := MustCapabilityKey[testCache]("cache.default", "example.com/contracts/cache", "Cache")
	seal := func(names ...string) string {
		catalog := New()
		for _, name := range names {
			catalog.MustRegister(capabilityDescriptor(name, key))
		}
		_, err := catalog.Seal()
		if err == nil {
			t.Fatal("duplicate provider accepted")
		}
		return err.Error()
	}
	first := seal("redis-b", "redis-a")
	second := seal("redis-a", "redis-b")
	if first != second {
		t.Fatalf("duplicate provider diagnostic depends on registration order:\nfirst=%q\nsecond=%q", first, second)
	}
}

func TestCapabilityExportFailsClosedWhenUndeclaredOrMissing(t *testing.T) {
	key := MustCapabilityKey[testCache]("cache.default", "example.com/contracts/cache", "Cache")
	module := capabilityTestModule{name: "redis-cache", exports: []CapabilityExport{ExportCapability(key, testCacheValue{})}}
	if _, err := CollectCapabilities([]Descriptor{{Name: "redis-cache", Build: func(BuildContext) (Instance, error) { return nil, nil }}}, []Instance{module}); err == nil || !strings.Contains(err.Error(), "declares none") {
		t.Fatalf("undeclared export error=%v", err)
	}
	if _, err := CollectCapabilities([]Descriptor{capabilityDescriptor("redis-cache", key)}, []Instance{capabilityTestModule{name: "redis-cache"}}); err == nil || !strings.Contains(err.Error(), "did not export declared capability") {
		t.Fatalf("missing export error=%v", err)
	}
}

func TestCapabilityResolveFailsClosedOnContractMismatch(t *testing.T) {
	providerKey := MustCapabilityKey[testCache]("cache.default", "example.com/contracts/cache", "Cache")
	module := capabilityTestModule{name: "redis-cache", exports: []CapabilityExport{ExportCapability(providerKey, testCacheValue{})}}
	set, err := CollectCapabilities([]Descriptor{capabilityDescriptor("redis-cache", providerKey)}, []Instance{module})
	if err != nil {
		t.Fatal(err)
	}
	consumerKey := MustCapabilityKey[testCache]("cache.default", "example.com/contracts/cache/v2", "Cache")
	if _, err := ResolveCapability(set, consumerKey); err == nil || !strings.Contains(err.Error(), "contract mismatch") {
		t.Fatalf("contract mismatch error=%v", err)
	}
}

func TestCapabilityResolveFailsClosedOnRuntimeTypeMismatch(t *testing.T) {
	wrongKey := MustCapabilityKey[string]("cache.default", "example.com/contracts/cache", "Cache")
	descriptor := Descriptor{
		Name:     "broken-cache",
		Provides: []CapabilityContract{wrongKey.Contract()},
		Build:    func(BuildContext) (Instance, error) { return nil, nil },
	}
	set, err := CollectCapabilities([]Descriptor{descriptor}, []Instance{capabilityTestModule{
		name:    "broken-cache",
		exports: []CapabilityExport{ExportCapability(wrongKey, "not-a-cache")},
	}})
	if err != nil {
		t.Fatal(err)
	}
	consumerKey := MustCapabilityKey[testCache]("cache.default", "example.com/contracts/cache", "Cache")
	if _, err := ResolveCapability(set, consumerKey); err == nil || !strings.Contains(err.Error(), "not assignable") {
		t.Fatalf("runtime type mismatch error=%v", err)
	}
}

func TestCapabilityContractValidationIsStaticAndDeterministic(t *testing.T) {
	if _, err := NewCapabilityKey[testCache]("Cache Default", "example.com/contracts/cache", "Cache"); err == nil {
		t.Fatal("invalid capability name accepted")
	}
	key := MustCapabilityKey[testCache]("cache.default", "example.com/contracts/cache", "Cache")
	if _, err := normalizeDescriptor(Descriptor{Name: "redis-cache", Provides: []CapabilityContract{key.Contract()}}); err == nil || !strings.Contains(err.Error(), "no Build function") {
		t.Fatalf("descriptor capability without Build error=%v", err)
	}
}
