// Package outboxruntime exposes the separately versioned infras plugin facade
// for Yunka's canonical transactional Outbox runtime.
package outboxruntime

import (
	"github.com/hvritual/yunka.io/framework/core/modulecatalog"
	canonical "github.com/hvritual/yunka.io/framework/modules/outboxruntime"
)

// ModuleName remains the canonical runtime module identity so moving to the
// infras distribution surface does not create a second module or configuration
// namespace.
const ModuleName = canonical.ModuleName

// GeneratedDescriptor returns the canonical Outbox runtime descriptor. The
// facade intentionally delegates implementation ownership to framework for the
// initial infras-module introduction; this preserves one Outbox runtime while
// giving applications a stable separately versioned infrastructure import path.
func GeneratedDescriptor() modulecatalog.Descriptor {
	return canonical.GeneratedDescriptor()
}
