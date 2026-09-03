package cache

// Cache is the fixture contract for the C10.3 typed infrastructure capability
// qualification. The provider implementation is intentionally owned outside
// the Application package so generated Assembly must import the declared Go
// contract rather than smuggling a concrete value through a factory closure.
type Cache interface {
	Prefix() string
}
