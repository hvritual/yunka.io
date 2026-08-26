package domain

func renderMultiPolicyStructural(spec Spec, packageImport string) map[string]string {
	files := map[string]string{
		"application/zz_yunka_service_gen.go":                     multiPolicyApplicationTemplate(spec, packageImport),
		"ports/zz_yunka_repositories_gen.go":                      multiPolicyPortsTemplate(spec, packageImport),
		"infrastructure/persistence/zz_yunka_repositories_gen.go": multiPolicyRepositoriesTemplate(spec, packageImport),
		"wire/zz_yunka_wiring_gen.go":                             multiPolicyWireTemplate(spec, packageImport),
	}
	for _, object := range spec.Objects {
		files["domain/zz_yunka_"+object.Name+"_entity_gen.go"] = multiEntityTemplate(spec, object)
		files["infrastructure/persistence/zz_yunka_"+object.Name+"_record_gen.go"] = multiRecordTemplate(spec, object, packageImport)
	}
	if spec.REST.Enabled {
		files["transport/rest/zz_yunka_rest_gen.go"] = multiPolicyRESTTemplate(spec, packageImport)
	}
	if spec.RPC.Enabled {
		files["transport/rpc/domain.proto"] = multiProtoTemplate(spec, packageImport)
		files["transport/rpc/zz_yunka_grpc_bridge_gen.go"] = multiPolicyGRPCBridgeTemplate(spec, packageImport)
	}
	return files
}
