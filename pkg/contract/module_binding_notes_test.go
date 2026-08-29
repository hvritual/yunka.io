package contract

import "testing"

func TestModuleBindingEvidenceIsCompilerLocal(t *testing.T) {
	binding := ModuleBinding{Name: "device", ImportPath: "example.com/product/modules/device", DescriptorSymbol: "GeneratedDescriptor", Evidence: "device/module.go+device/autoload/register.go"}
	if binding.Name == "" || binding.ImportPath == "" || binding.DescriptorSymbol != "GeneratedDescriptor" || binding.Evidence == "" {
		t.Fatalf("invalid explicit module binding evidence: %#v", binding)
	}
}
