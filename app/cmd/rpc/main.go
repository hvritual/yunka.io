package main

import (
	"flag"
	"fmt"
	"os"

	bmgen "yunka-rpc/generator"
	"yunka-rpc/protobuf/pkg/gen"
	"yunka-rpc/protobuf/pkg/generator"
)

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *versionFlag {
		fmt.Println(generator.Version)
		os.Exit(0)
	}

	g := bmgen.MigGenerator()
	gen.Main(g)
}
