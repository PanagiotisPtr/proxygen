package main

import (
	"fmt"
	"golang.org/x/tools/go/packages"
)

func main() {
	fmt.Println("hello")

	packagePath := "github.com/panagiotisptr/service-proxies-demo/service"
	interfaceName := "TaskService"

	fset := token.NewFileSet()
	pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedSyntax}, packagePath)

}
