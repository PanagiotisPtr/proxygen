package main

import (
	"flag"

	"github.com/panagiotisptr/proxygen/generate"
)

func main() {
	// flags
	interfacePath := flag.String("interface", "", "interface full path - {package}.{interface}")
	packageName := flag.String("package", "", "package name")
	name := flag.String("name", "", "name of the generated proxy struct")
	output := flag.String("output", "", "output file name")

	flag.Parse()

	generate.LoadPackage(
		*interfacePath,
		*packageName,
		*name,
		*output,
	)
}
