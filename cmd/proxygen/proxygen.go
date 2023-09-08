package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/panagiotisptr/proxygen/generate"
)

func main() {
	// flags
	interfacePath := flag.String("interface", "", "interface full path - {package}.{interface}")
	packageName := flag.String("package", "", "package name")
	name := flag.String("name", "", "name of the generated proxy struct")
	output := flag.String("output", "", "output file name")

	flag.Parse()

	generator := generate.NewGenerator()

	err := generator.GenerateProxy(
		*interfacePath,
		*packageName,
		*name,
		*output,
	)
	if err != nil {
		fmt.Println("encountered error while generating proxy:", err)
		os.Exit(1)
	}
	fmt.Println("proxy generated successfully")
}
