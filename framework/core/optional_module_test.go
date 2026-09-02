package core

import (
	"testing"

	"gorm.io/gorm"
	"github.com/hvritual/yunka.io/framework/core/modulecatalog"
)

type optionalModuleDatabases struct{}

func (optionalModuleDatabases) GORM(name string) (*gorm.DB, error) {
	return &gorm.DB{}, nil
}

func TestNewAppValidatesDeclarativeModuleRequirementsWithoutBuildingInstance(t *testing.T) {
	catalog := modulecatalog.New()
	catalog.MustRegister(modulecatalog.Descriptor{
		Name:         "access",
		Requirements: modulecatalog.Requirements{Databases: []modulecatalog.DatabaseRequirement{{Name: "primary"}}},
	})
	application, err := NewApp(AppOptions{Catalog: catalog, Databases: optionalModuleDatabases{}})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(application.composedModuleSnapshot()); got != 0 {
		t.Fatalf("declarative module created runtime instance count=%d", got)
	}
}

func TestNewAppRejectsMissingDeclarativeModuleCapability(t *testing.T) {
	catalog := modulecatalog.New()
	catalog.MustRegister(modulecatalog.Descriptor{
		Name:         "access",
		Requirements: modulecatalog.Requirements{Databases: []modulecatalog.DatabaseRequirement{{Name: "primary"}}},
	})
	if _, err := NewApp(AppOptions{Catalog: catalog}); err == nil {
		t.Fatal("missing declarative database capability accepted")
	}
}
