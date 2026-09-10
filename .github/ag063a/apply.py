from pathlib import Path


def one_replace(path: str, old: str, new: str) -> None:
    p = Path(path)
    data = p.read_text()
    count = data.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one replacement target, found {count}: {old[:80]!r}")
    p.write_text(data.replace(old, new, 1))


plan = Path("app/cmd/implementation/plan.go")
data = plan.read_text()
start = data.index("\tif len(selected.Application.Requires) > 0 || len(selected.Application.Capabilities) > 0 {")
end = data.index("\t// Use the existing compiler/renderer's exact artifacts", start)
replacement = '''\tif len(selected.Application.Capabilities) > 0 {
\t\treturn Report{}, fmt.Errorf("add implementation: infrastructure-capability Applications require an explicit capability template; refusing to omit dependencies")
\t}
\tif err := validateDependencyOperations(manifest, *selected); err != nil {
\t\treturn Report{}, err
\t}
'''
data = data[:start] + replacement + data[end:]
plan.write_text(data)
one_replace(
    str(plan),
    '''\tport, err := selectPort(generated, *selected)\n\tif err != nil {\n\t\treturn Report{}, err\n\t}\n''',
    '''\tport, err := selectPort(generated, *selected)\n\tif err != nil {\n\t\treturn Report{}, err\n\t}\n\tcontractImport := project.GeneratedGoImport + "/" + domain + "/application"\n\tdependencies, err := selectDependencies(generated, manifest, *selected, port, contractImport)\n\tif err != nil {\n\t\treturn Report{}, err\n\t}\n''',
)
one_replace(
    str(plan),
    '\tcontents, err := starterFiles(port, ownerImport, project.GeneratedGoImport+"/"+domain+"/application", key, caller)\n',
    '\tcontents, err := starterFilesWithDependencies(port, ownerImport, contractImport, key, caller, dependencies)\n',
)
one_replace(
    str(plan),
    'Contract: project.GeneratedGoImport + "/" + domain + "/application." + port.name',
    'Contract: contractImport + "." + port.name',
)

render = "app/cmd/implementation/render.go"
one_replace(
    render,
    'func starterFiles(port portShape, owner, contractImport, key, caller string) (map[string][]byte, error) {\n',
    'func starterFiles(port portShape, owner, contractImport, key, caller string) (map[string][]byte, error) {\n\treturn starterFilesWithDependencies(port, owner, contractImport, key, caller, nil)\n}\n\nfunc starterFilesWithDependencies(port portShape, owner, contractImport, key, caller string, dependencies []dependencyShape) (map[string][]byte, error) {\n',
)
one_replace(
    render,
    '''\townerBody := fmt.Sprintf("package owner\\nimport ( _port %q; _impl %q )\\n\\n// Build is a resource-free composition entry point, not a business invocation API.\\nfunc Build() _port.%s {return _impl.New()}\\n", contractImport, owner+"/internal/usecase", port.name)\n''',
    '''\townerBody := fmt.Sprintf("package owner\\nimport ( _port %q; _impl %q )\\n\\n// Build is a resource-free composition entry point, not a business invocation API.\\nfunc Build() _port.%s {return _impl.New()}\\n", contractImport, owner+"/internal/usecase", port.name)
\tif len(dependencies) > 0 {
\t\tparams := make([]string, 0, len(dependencies))
\t\targs := make([]string, 0, len(dependencies))
\t\tfor i, dependency := range dependencies {
\t\t\tparams = append(params, fmt.Sprintf("dependency%d _port.%s", i, dependency.ContractName))
\t\t\targs = append(args, fmt.Sprintf("dependency%d", i))
\t\t}
\t\townerBody = fmt.Sprintf("package owner\\nimport ( _port %q; _impl %q )\\n\\n// Build accepts only generated source-edge child capabilities.\\nfunc Build(%s) (_port.%s,error) {return _impl.New(%s)}\\n", contractImport, owner+"/internal/usecase", strings.Join(params, ","), port.name, strings.Join(args, ","))
\t}
''',
)
one_replace(render, "\tvar fields, delegates strings.Builder\n", "\tvar fields, delegates, initializers strings.Builder\n")

p = Path(render)
data = p.read_text()
start = data.index("\t\timports := neededImports(fn, port.imports)")
end = data.index("\t\tctxType, err := expression(fn.Params.List[0].Type)", start)
handler = '''\t\timports := neededImports(fn, port.imports)
\t\tvar dependencyFields strings.Builder
\t\tdependencyAlias := ""
\t\tfor dependencyIndex, dependency := range dependencies {
\t\t\tif !dependency.Uses(name) {
\t\t\t\tcontinue
\t\t\t}
\t\t\tif dependencyAlias == "" {
\t\t\t\tdependencyAlias = freshAlias("_starterdependency", imports)
\t\t\t\timports[dependencyAlias] = dependency.ContractImport
\t\t\t}
\t\t\tfmt.Fprintf(&dependencyFields, "dependency%d %s.%s\\n", dependencyIndex, dependencyAlias, dependency.ContractName)
\t\t}
\t\terrAlias := freshAlias("_startererrors", imports)
\t\timports[errAlias] = "errors"
\t\tif dependencyFields.Len() == 0 {
\t\t\tbody := fmt.Sprintf("package usecase\\nimport(\\n%s)\\n\\n// handle%s owns this use case only. Add narrow dependencies here, not to the facade.\\ntype handle%s struct{}\\n\\nfunc (h *handle%s) execute%s {return nil,%s.New(%q)}\\n", importLines(imports), name, name, name, signature, errAlias, key+"."+name+": not implemented")
\t\t\tif err := addGo("internal/usecase/" + filename, body); err != nil {
\t\t\t\treturn nil, err
\t\t\t}
\t\t} else {
\t\t\tbody := fmt.Sprintf("package usecase\\nimport(\\n%s)\\n\\n// handle%s owns this use case only. Dependencies are exact source-edge child capabilities.\\ntype handle%s struct{\\n%s}\\n\\nfunc (h *handle%s) execute%s {return nil,%s.New(%q)}\\n", importLines(imports), name, name, dependencyFields.String(), name, signature, errAlias, key+"."+name+": not implemented")
\t\t\tif err := addGo("internal/usecase/" + filename, body); err != nil {
\t\t\t\treturn nil, err
\t\t\t}
\t\t}
\t\tfmt.Fprintf(&fields, "h%d handle%s\\n", i, name)
\t\tfmt.Fprintf(&initializers, "h%d: handle%s{", i, name)
\t\tfor dependencyIndex, dependency := range dependencies {
\t\t\tif dependency.Uses(name) {
\t\t\t\tfmt.Fprintf(&initializers, "dependency%d: dependency%d,", dependencyIndex, dependencyIndex)
\t\t\t}
\t\t}
\t\tinitializers.WriteString("},")
'''
data = data[:start] + handler + data[end:]
p.write_text(data)

one_replace(
    render,
    '''\twiring := fmt.Sprintf("package usecase\\nimport(\\n%s)\\n\\ntype service struct {\\n%s}\\n\\n// New exposes only the canonical interface; never return or unwrap service.\\nfunc New() %s.%s {return &service{}}\\n\\nvar _ %s.%s = (*service)(nil)\\n\\n%s", importLines(allImports), fields.String(), portAlias, port.name, portAlias, port.name, delegates.String())\n''',
    '''\tvar wiring string
\tif len(dependencies) == 0 {
\t\twiring = fmt.Sprintf("package usecase\\nimport(\\n%s)\\n\\ntype service struct {\\n%s}\\n\\n// New exposes only the canonical interface; never return or unwrap service.\\nfunc New() %s.%s {return &service{}}\\n\\nvar _ %s.%s = (*service)(nil)\\n\\n%s", importLines(allImports), fields.String(), portAlias, port.name, portAlias, port.name, delegates.String())
\t} else {
\t\terrAlias := freshAlias("_startererrors", allImports)
\t\tallImports[errAlias] = "errors"
\t\tparams := make([]string, 0, len(dependencies))
\t\tvar checks strings.Builder
\t\tfor i, dependency := range dependencies {
\t\t\tparams = append(params, fmt.Sprintf("dependency%d %s.%s", i, portAlias, dependency.ContractName))
\t\t\tfmt.Fprintf(&checks, "if dependency%d == nil {return nil,%s.New(%q)}\\n", i, errAlias, key+": dependency "+dependency.Key+" is required")
\t\t}
\t\twiring = fmt.Sprintf("package usecase\\nimport(\\n%s)\\n\\ntype service struct {\\n%s}\\n\\n// New accepts only generated source-edge child capabilities and exposes only the canonical interface.\\nfunc New(%s) (%s.%s,error) {%sreturn &service{%s},nil}\\n\\nvar _ %s.%s = (*service)(nil)\\n\\n%s", importLines(allImports), fields.String(), strings.Join(params, ","), portAlias, port.name, checks.String(), initializers.String(), portAlias, port.name, delegates.String())
\t}
''',
)
one_replace(
    render,
    '''\tpolicy := applicationboundary.Policy{SchemaVersion: applicationboundary.SchemaVersion, Factories: []applicationboundary.Factory{{Symbol: applicationboundary.Symbol{Package: owner, Name: "Build"}, AllowedCallers: []string{caller}, Results: []applicationboundary.Slot{{Index: 0, Contract: applicationboundary.Symbol{Package: contractImport, Name: port.name}}}}}}\n''',
    '''\tfactory := applicationboundary.Factory{Symbol: applicationboundary.Symbol{Package: owner, Name: "Build"}, AllowedCallers: []string{caller}, Results: []applicationboundary.Slot{{Index: 0, Contract: applicationboundary.Symbol{Package: contractImport, Name: port.name}}}}
\tfor i, dependency := range dependencies {
\t\tfactory.Arguments = append(factory.Arguments, applicationboundary.Slot{Index: i, Contract: applicationboundary.Symbol{Package: dependency.ContractImport, Name: dependency.ContractName}})
\t}
\tpolicy := applicationboundary.Policy{SchemaVersion: applicationboundary.SchemaVersion, Factories: []applicationboundary.Factory{factory}}
''',
)
one_replace(
    render,
    '\tfiles["README.md"] = []byte(fmt.Sprintf(`# %s implementation starter\n',
    '\tdependencyNote := "Cross-Application/infrastructure-dependent templates are a later AG-06 task."\n\tif len(dependencies) > 0 {\n\t\tdependencyNote = "Canonical Application `requires` edges use generated source-edge ChildCapability interfaces; infrastructure capability templates remain unsupported and are never guessed."\n\t}\n\tfiles["README.md"] = []byte(fmt.Sprintf(`# %s implementation starter\n',
)
one_replace(render, "Cross-Application/infrastructure-dependent templates are a later AG-06 task.\n\nValidate using the existing commands", "%s\n\nValidate using the existing commands")
one_replace(render, "`, key, contractImport, port.name, caller))", "`, key, contractImport, port.name, caller, dependencyNote))")
