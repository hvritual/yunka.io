package architecturepolicy

// All fixtures are repository-owned synthetic examples. They deliberately use
// no Biz/Delivery types and no external dependencies. This is mechanism evidence,
// not a claim that those consumers contain every demonstrated leak.
type agBoundaryCase struct {
	name, output, diagnostic string
	files                    map[string]string
}

func agBoundaryCases() []agBoundaryCase {
	store := `package store
type Store struct{}
func (*Store) Read() string { return "read-ok" }
func (*Store) Delete() {}
`
	owner := func(body string) string {
		return "package a\nimport \"" + agBoundaryModule + "/internal/a/internal/store\"\n" + body
	}
	main := func(body string) string {
		return "package main\nimport (\"fmt\"; \"" + agBoundaryModule + "/internal/a\")\nfunc main() { " + body + " }\n"
	}
	fixture := func(build, probe string) map[string]string {
		return map[string]string{
			"internal/a/internal/store/store.go": store,
			"internal/a/build.go":                build,
			"cmd/probe/main.go":                  probe,
		}
	}
	reader := "type Reader interface { Read() string }\n"
	leak := `r := a.NewReader(); if r.Read() != "read-ok" { panic("read failed") }; d, ok := r.(interface{ Delete() }); if !ok { panic("expected leak characterization") }; d.Delete(); fmt.Println("extra-method-exposed")`
	narrow := `r := a.NewReader(); if r.Read() != "read-ok" { panic("read failed") }; if _, ok := r.(interface{ Delete() }); ok { panic("method leaked") }; fmt.Println("extra-method-not-exposed")`
	privateWrapper := reader + `type readerView struct { target *store.Store }
func (r readerView) Read() string { return r.target.Read() }
func NewReader() Reader { return readerView{target: &store.Store{}} }
`
	cases := []agBoundaryCase{
		{
			name: "owner_factory", output: "read-ok",
			files: fixture(owner("func Read() string { return (&store.Store{}).Read() }\n"), main("fmt.Println(a.Read())")),
		},
		{
			name: "root_internal_allows_sibling", output: "read-ok",
			files: map[string]string{
				"internal/a/store/store.go": store,
				"internal/b/b.go":          "package b\nimport renamed \"" + agBoundaryModule + "/internal/a/store\"\nfunc Read() string { return (&renamed.Store{}).Read() }\n",
				"cmd/probe/main.go":        "package main\nimport (\"fmt\"; \"" + agBoundaryModule + "/internal/b\")\nfunc main() { fmt.Println(b.Read()) }\n",
			},
		},
		{
			name: "narrow_interface_wide_object", output: "extra-method-exposed",
			files: fixture(owner(reader+"func NewReader() Reader { return &store.Store{} }\n"), main(leak)),
		},
		{
			name: "private_explicit_wrapper", output: "extra-method-not-exposed",
			files: fixture(owner(privateWrapper), main(narrow)),
		},
		{
			name: "concrete_return_leaks_without_import", output: "raw-value-exposed",
			files: fixture(owner("func New() *store.Store { return &store.Store{} }\n"), main(`s := a.New(); s.Delete(); fmt.Println("raw-value-exposed")`)),
		},
		{
			name: "embedding_promotes_extra_method", output: "extra-method-exposed",
			files: fixture(owner(reader+"type view struct { *store.Store }\nfunc NewReader() Reader { return view{&store.Store{}} }\n"), main(leak)),
		},
		{
			name: "public_unwrap_leaks", output: "unwrapped-value-exposed",
			files: fixture(owner(privateWrapper+"func (r readerView) Unwrap() interface{ Delete() } { return r.target }\n"), main(`r := a.NewReader(); u := r.(interface{ Unwrap() interface{ Delete() } }); u.Unwrap().Delete(); fmt.Println("unwrapped-value-exposed")`)),
		},
		{
			name: "same_package_cross_file_access", output: "same-package-private-access",
			files: fixture(owner(privateWrapper), main(`a.Inspect(); fmt.Println("same-package-private-access")`)),
		},
		{
			name:       "different_package_private_field_rejected",
			diagnostic: `^cmd/probe/main\.go:[0-9]+:[0-9]+: .*target.*(unexported|undefined).*$`,
			files: fixture(owner(`type View struct { target *store.Store }
func New() *View { return &View{target: &store.Store{}} }
`), main(`v := a.New(); v.target.Delete(); fmt.Println("unexpected")`)),
		},
	}
	cases[7].files["internal/a/inspect.go"] = "package a\nfunc Inspect() { NewReader().(readerView).target.Delete() }\n"
	for _, alias := range []string{"store", "renamed"} {
		cases = append(cases, agBoundaryCase{
			name:       "nested_internal_rejects_" + alias,
			diagnostic: `^internal/b/b\.go:[0-9]+:[0-9]+: use of internal package example\.com/agboundary/internal/a/internal/store not allowed$`,
			files: map[string]string{
				"internal/a/internal/store/store.go": store,
				"internal/b/b.go":                   "package b\nimport " + alias + " \"" + agBoundaryModule + "/internal/a/internal/store\"\nfunc Read() string { return (&" + alias + ".Store{}).Read() }\n",
				"cmd/probe/main.go":                 "package main\nimport (\"fmt\"; \"" + agBoundaryModule + "/internal/b\")\nfunc main() { fmt.Println(b.Read()) }\n",
			},
		})
	}
	return cases
}
