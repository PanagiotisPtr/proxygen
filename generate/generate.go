package generate

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/panagiotisptr/proxygen/templates"
	"golang.org/x/tools/go/packages"
)

type ImportData struct {
	Path          string
	Name          string
	Alias         string
	PkgName       string
	InterfaceName string
	Used          bool
}

func (id ImportData) Selector() string {
	if id.Alias != "" {
		return id.Alias
	}

	return id.Name
}

type MethodParam string

func (m MethodParam) IsVariadic() bool {
	return strings.HasPrefix(string(m), "...")
}

func (m MethodParam) Type() string {
	if m.IsVariadic() {
		return "[]" + strings.TrimPrefix(string(m), "...")
	}

	return string(m)
}

type MethodData struct {
	Name   string
	Params []MethodParam
	Rets   []string
}

type InterfaceData struct {
	InterfacePackage    string
	InterfaceName       string
	Imports             []*ImportData
	Methods             []*MethodData
	ImplementationType  string
	OriginalPackageName string
}

const mode packages.LoadMode = packages.NeedName |
	packages.NeedTypes |
	packages.NeedSyntax |
	packages.NeedTypesInfo |
	packages.NeedImports

type Generator struct {
	cfg *packages.Config
}

func NewGenerator() *Generator {
	return &Generator{
		cfg: &packages.Config{
			Fset: token.NewFileSet(),
			Mode: mode,
			Dir:  ".",
		},
	}
}

func (g *Generator) GenerateProxy(
	interfacePath string,
	packageName string,
	name string,
	output string,
) error {
	sections := strings.Split(interfacePath, ".")
	packagePath := strings.Join(sections[:len(sections)-1], ".")
	interfaceName := sections[len(sections)-1]

	data, err := g.getInterfaceData(packagePath, interfaceName, packageName)
	if err != nil {
		return err
	}

	// fix the imports for the main package
	if data.OriginalPackageName != packageName {
		for _, m := range data.Imports {
			if m.Path == packagePath {
				m.Used = true
				break
			}
		}
	}

	usedImports := []*ImportData{}
	for _, imp := range data.Imports {
		if !imp.Used {
			continue
		}
		usedImports = append(usedImports, imp)
	}
	data.Imports = usedImports

	// ensure deterministic output PER file
	for i, imp := range usedImports {
		newAlias := fmt.Sprintf("import%s%s%d", imp.PkgName, imp.InterfaceName, i)
		for _, meth := range data.Methods {
			for parami := range meth.Params {
				meth.Params[parami] = MethodParam(strings.ReplaceAll(string(meth.Params[parami]), imp.Alias, newAlias))
			}
			for reti := range meth.Rets {
				meth.Rets[reti] = strings.ReplaceAll(meth.Rets[reti], imp.Alias, newAlias)
			}
		}
		imp.Alias = newAlias
	}

	// render template
	tmpl := template.Must(template.New("proxy").Parse(templates.ProxyTemplate))
	var generatedProxy bytes.Buffer
	err = tmpl.Execute(&generatedProxy, struct {
		PackageName string
		Name        string
		InterfaceData
	}{
		PackageName:   packageName,
		Name:          name,
		InterfaceData: data,
	})
	if err != nil {
		return err
	}

	// foramt generated proxy
	formattedContent, formatErr := format.Source(generatedProxy.Bytes())
	if formatErr != nil {
		return fmt.Errorf("error formatting generated proxy: %w", formatErr)
	}

	// write file
	f, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer f.Close()
	_, err = f.Write(formattedContent)
	if err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	return nil
}

func (g *Generator) getInterfaceData(
	interfacePackage string,
	interfaceName string,
	outputPkgName string,
) (InterfaceData, error) {
	data := InterfaceData{
		InterfacePackage: interfacePackage,
		InterfaceName:    interfaceName,
		Imports:          []*ImportData{},
	}
	existingImports := []*ImportData{}
	newImports := []*ImportData{}

	pkg, err := g.getPackage(interfacePackage)
	if err != nil {
		return data, err
	}

	// load imports
	newImports = append(newImports, &ImportData{
		Path: interfacePackage,
		Name: pkg.Name,
	})
	keys := make([]string, len(pkg.Imports))
	i := 0
	for k := range pkg.Imports {
		keys[i] = k
		i++
	}
	// sort imports to ensure that generated code is deterministic
	sort.Strings(keys)
	for i := range keys {
		imp := pkg.Imports[keys[i]]
		existingImports = append(existingImports, &ImportData{
			Path: imp.PkgPath,
			Name: imp.Name,
		})
		newImports = append(newImports, &ImportData{
			Path: imp.PkgPath,
			Name: imp.Name,
		})
	}

	// useful later to determine wether or not this package was used
	for i := range newImports {
		// alias all new imports so that they won't conflict with anything we might add
		// later - which could be from merging embedded interface imports
		newImports[i].Alias = fmt.Sprintf("import%s%s%d", pkg.Name, interfaceName, i)
		newImports[i].PkgName = pkg.Name
		newImports[i].InterfaceName = interfaceName
	}

	var interfaceFile *ast.File
	for _, fileAst := range pkg.Syntax {
		interfaceFile = fileAst
		ifaceTypeSpec, ifaceErr := g.getInterface(fileAst, interfaceName)
		if ifaceErr != nil {
			continue
		}
		iface := ifaceTypeSpec.Type.(*ast.InterfaceType)
		ifaceIdent := ifaceTypeSpec.Name
		// update aliases of existing imports
		ast.Inspect(fileAst, func(n ast.Node) bool {
			switch t := n.(type) {
			case *ast.ImportSpec:
				// check if it's import
				if t.Name != nil {
					for i, imp := range existingImports {
						p := strings.ReplaceAll(t.Path.Value, "\"", "")
						if imp.Path == p {
							existingImports[i].Alias = t.Name.Name
						}
					}
				}
				return false
			}

			return true
		})
		data.Imports = newImports

		fset := g.cfg.Fset
		filename := fset.Position(interfaceFile.Package).Filename
		content, _ := os.ReadFile(filename)

		tp, err := NewTypeProcessor(pkg, g.cfg.Fset, filename)
		if err != nil {
			return data, err
		}

		addSelectorToLocals := pkg.Name != outputPkgName
		for _, m := range iface.Methods.List {
			if m.Names == nil {
				continue
			}

			methodData := &MethodData{
				Name: m.Names[0].Name,
			}

			if m.Type != nil {
				switch t := m.Type.(type) {
				case *ast.FuncType:
					if t.Params != nil {
						for _, param := range t.Params.List {
							for i := 0; i < max(1, len(param.Names)); i++ {
								methodData.Params = append(
									methodData.Params,
									MethodParam(tp.correctType(
										param.Type,
										existingImports,
										newImports,
										interfacePackage,
										addSelectorToLocals,
									)),
								)
							}
						}
					}

					if t.Results != nil {
						for _, result := range t.Results.List {
							for i := 0; i < max(1, len(result.Names)); i++ {
								methodData.Rets = append(
									methodData.Rets,
									tp.correctType(
										result.Type,
										existingImports,
										newImports,
										interfacePackage,
										addSelectorToLocals,
									),
								)
							}
						}
					}
				}
			}

			data.Methods = append(data.Methods, methodData)
		}
		data.ImplementationType = tp.correctType(
			ifaceIdent,
			existingImports,
			newImports,
			interfacePackage,
			addSelectorToLocals,
		)

		// used later to make sure it's imported if needed
		data.OriginalPackageName = pkg.Name

		queue := [][2]string{}
		ast.Inspect(iface, func(n ast.Node) bool {
			switch t := n.(type) {
			case *ast.Field:
				// we know that all embedded interfaces don't have names
				if len(t.Names) == 0 {
					return true
				}
				return false
			case *ast.Ident:
				queue = append(queue, [2]string{interfacePackage, t.Name})
				return false
			case *ast.SelectorExpr:
				selectorStart := fset.Position(t.X.Pos())
				selectorEnd := fset.Position(t.X.End())
				selectorPkg := string(content[selectorStart.Offset:selectorEnd.Offset])
				for _, i := range existingImports {
					if i.Selector() == selectorPkg {
						for idx := range newImports {
							if newImports[idx].Path == i.Path {
								queue = append(queue, [2]string{newImports[idx].Path, t.Sel.Name})
								return false
							}
						}
					}
				}
				return false
			}

			return true
		})

		for _, embeddedIface := range queue {
			embeddedData, embeddedErr := g.getInterfaceData(
				embeddedIface[0],
				embeddedIface[1],
				outputPkgName,
			)
			if embeddedErr != nil {
				continue
			}

			data.Imports = append(data.Imports, embeddedData.Imports...)
			data.Methods = append(data.Methods, embeddedData.Methods...)
		}
	}

	return data, nil
}

func (g *Generator) getPackage(pkgPath string) (
	*packages.Package,
	error,
) {
	pkgs, err := packages.Load(g.cfg, pkgPath)
	if err != nil {
		return nil, err
	}

	var pkg *packages.Package
	for _, p := range pkgs {
		if p.PkgPath == pkgPath {
			pkg = p
			break
		}
	}

	if pkg == nil {
		return nil, fmt.Errorf("package %s not found", pkgPath)
	}

	return pkg, nil
}

func (g *Generator) getInterface(fileAst *ast.File, name string) (*ast.TypeSpec, error) {
	var rv *ast.TypeSpec
	ast.Inspect(fileAst, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.TypeSpec:
			if !t.Name.IsExported() {
				return false
			}
			if t.Name.Name != name {
				return false
			}
			if _, ok := t.Type.(*ast.InterfaceType); ok {
				rv = t
				return false
			}
		}

		return true
	})
	if rv == nil {
		return nil, fmt.Errorf("interface %s not found", name)
	}

	return rv, nil
}
