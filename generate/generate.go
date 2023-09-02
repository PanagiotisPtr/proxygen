package generate

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"reflect"
	"strings"
	"text/template"

	"golang.org/x/tools/go/packages"
)

type ImportData struct {
	Path  string
	Name  string
	Alias string
	Used  bool
}

func (id ImportData) Selector() string {
	if id.Alias != "" {
		return id.Alias
	}

	return id.Name
}

type MethodData struct {
	Name   string
	Params []string
	Rets   []string
}

type InterfaceData struct {
	InterfacePackage   string
	InterfaceName      string
	Imports            []ImportData
	Methods            []MethodData
	ImplementationType string
}

const mode packages.LoadMode = packages.NeedName |
	packages.NeedTypes |
	packages.NeedSyntax |
	packages.NeedTypesInfo |
	packages.NeedImports

func LoadPackage(
	interfacePath string,
	packageName string,
	name string,
	output string,
) {
	sections := strings.Split(interfacePath, ".")
	packagePath := strings.Join(sections[:len(sections)-1], ".")
	interfaceName := sections[len(sections)-1]

	fmt.Println(packagePath)
	fmt.Println(interfaceName)

	var fset = token.NewFileSet()
	cfg := &packages.Config{Fset: fset, Mode: mode, Dir: "."}

	data, err := getInterfaceData(fset, cfg, packagePath, interfaceName, []ImportData{})
	if err != nil {
		fmt.Println("err: ", err)
		os.Exit(1)
	}

	//render template
	tmpl := template.Must(template.New("proxy").Parse(t))
	var buf bytes.Buffer
	er := tmpl.Execute(&buf, struct {
		PackageName string
		Name        string
		InterfaceData
	}{
		PackageName:   packageName,
		Name:          name,
		InterfaceData: data,
	})

	generatedProxy := buf
	fmt.Println("buf: ", generatedProxy.String())
	fmt.Println("err: ", er)

	// foramt generated proxy
	bs, e := format.Source(generatedProxy.Bytes())
	fmt.Println("formatted: ", string(bs))
	fmt.Println("err: ", e)

	// write file
	f, err := os.Create(output)
	if err != nil {
		fmt.Println("error writing file: ", err)
		os.Exit(1)
	}
	defer f.Close()
	_, err = f.Write(bs)
	if err != nil {
		fmt.Println("error writing file: ", err)
		os.Exit(1)
	}

}

func getInterfaceData(
	fset *token.FileSet,
	cfg *packages.Config,
	interfacePackage string,
	interfaceName string,
	imports []ImportData,
) (InterfaceData, error) {
	data := InterfaceData{
		InterfacePackage: interfacePackage,
		InterfaceName:    interfaceName,
		Imports:          imports,
	}
	pkgs, err := packages.Load(cfg, interfacePackage)
	if err != nil {
		return data, err
	}

	defs := make(map[string]types.Object)
	existingImports := []ImportData{}
	newImports := []ImportData{}
	var pkg *packages.Package
	for _, p := range pkgs {
		if p.PkgPath == interfacePackage {
			pkg = p
			newImports = append(newImports, ImportData{
				Path: interfacePackage,
				Name: p.Name,
			})
			for _, imp := range p.Imports {
				existingImports = append(existingImports, ImportData{
					Path: imp.PkgPath,
					Name: imp.Name,
				})
				newImports = append(newImports, ImportData{
					Path: imp.PkgPath,
					Name: imp.Name,
				})
			}
			// print all types in package
			for _, t := range p.TypesInfo.Defs {
				if t == nil {
					continue
				}
				if _, ok := t.(*types.TypeName); !ok {
					continue
				}
				if !t.Exported() {
					continue
				}
				if _, ok := defs[t.Name()]; ok {
					continue
				}
				fmt.Println("DEFS:", t.Name(), "from package:", t.Pkg().Path(), "type:", reflect.TypeOf(t))
				defs[t.Name()] = t
			}
		}
	}
	for i := range newImports {
		newImports[i].Alias = fmt.Sprintf("import%d", i)
	}

	var iface *ast.InterfaceType
	var ifaceIdent *ast.Ident
	var f *ast.File

	for _, fileAst := range pkg.Syntax {
		f = fileAst
		foundIface := false
		ast.Inspect(fileAst, func(n ast.Node) bool {
			switch t := n.(type) {
			case *ast.TypeSpec:
				if !t.Name.IsExported() {
					return false
				}
				if t.Name.Name != interfaceName {
					return false
				}
				ifaceIdent = t.Name
				switch ti := t.Type.(type) {
				case *ast.InterfaceType:
					iface = ti
					foundIface = true
					return false
				default:
					return false
				}
			}

			return true
		})
		if !foundIface {
			continue
		}
		fmt.Println("file: ", fileAst.Name)
		ast.Inspect(fileAst, func(n ast.Node) bool {
			switch t := n.(type) {
			case *ast.ImportSpec:
				// check if it's import
				path := strings.ReplaceAll(t.Path.Value, "\"", "")
				fmt.Println("importAst: ", t)
				fmt.Println("import: ", path, "as", t.Name)
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

		fmt.Println("processed imports: ")
		for _, imp := range existingImports {
			fmt.Println("import: ", imp.Path, "name:", imp.Name, "alias:", imp.Alias)
		}

		fmt.Println("new imports: ")
		for _, imp := range newImports {
			fmt.Println("import: ", imp.Path, "name:", imp.Name, "alias:", imp.Alias)
		}
		data.Imports = newImports

		filename := fset.Position(f.Package).Filename
		fmt.Println("filepath: ", filename)
		content, _ := os.ReadFile(filename)
		start := fset.Position(iface.Pos())
		end := fset.Position(iface.End())
		fmt.Println("interface: ", string(content[start.Offset:end.Offset]))

		var realType func(t ast.Expr) *ast.Ident
		realType = func(t ast.Expr) *ast.Ident {
			switch t := t.(type) {
			case *ast.Ident:
				return t
			case *ast.StarExpr:
				return realType(t.X)
			case *ast.SelectorExpr:
				return realType(t.Sel)
			case *ast.ArrayType:
				return realType(t.Elt)
			case *ast.MapType:
				return realType(t.Value)
			case *ast.ChanType:
				return realType(t.Value)
			default:
				return nil
			}
		}

		var correctedType func(t ast.Expr) string
		correctedType = func(t ast.Expr) string {
			switch t := t.(type) {
			case *ast.Ident:
				for _, defs := range defs {
					if defs.Name() == t.Name {
						for i := range newImports {
							if newImports[i].Path == interfacePackage {
								newImports[i].Used = true
								return newImports[i].Alias + "." + t.Name
							}
						}
					}
				}
				return t.Name
			case *ast.StarExpr:
				return "*" + correctedType(t.X)
			case *ast.SelectorExpr:
				selectorStart := fset.Position(t.X.Pos())
				selectorEnd := fset.Position(t.X.End())
				selectorPkg := string(content[selectorStart.Offset:selectorEnd.Offset])
				for _, i := range existingImports {
					if i.Selector() == selectorPkg {
						for idx := range newImports {
							if newImports[idx].Path == i.Path {
								newImports[idx].Used = true
								return newImports[idx].Alias + "." + t.Sel.Name
							}
						}
					}
				}
				return selectorPkg + "." + realType(t.Sel).Name
			case *ast.ArrayType:
				return "[]" + correctedType(t.Elt)
			case *ast.MapType:
				return "map[" + correctedType(t.Key) + "]" + correctedType(t.Value)
			case *ast.ChanType:
				// channel with correct arrow position
				if t.Arrow == token.NoPos {
					return "chan " + correctedType(t.Value)
				} else if t.Arrow == token.Pos(start.Offset) {
					return "<-chan " + correctedType(t.Value)
				} else {
					return "chan<- " + correctedType(t.Value)
				}
			default:
				return ""
			}
		}

		for _, m := range iface.Methods.List {
			start = fset.Position(m.Pos())
			end = fset.Position(m.End())
			fmt.Println("method: ", string(content[start.Offset:end.Offset]))

			if m.Names == nil {
				continue
			}

			methodData := MethodData{
				Name: m.Names[0].Name,
			}
			fmt.Println("methodData: ", methodData)

			if m.Type != nil {
				switch t := m.Type.(type) {
				case *ast.FuncType:
					if t.Params != nil {
						for _, param := range t.Params.List {
							methodData.Params = append(
								methodData.Params,
								correctedType(param.Type),
							)
						}
					}

					if t.Results != nil {
						for _, result := range t.Results.List {
							methodData.Rets = append(
								methodData.Rets,
								correctedType(result.Type),
							)
						}
					}
				}
			}

			data.Methods = append(data.Methods, methodData)
		}
		data.ImplementationType = correctedType(ifaceIdent)
	}

	return data, nil
}

const t = `// Code generated by proxygen. DO NOT EDIT.
package {{ .PackageName }}

import (
    proxygenInterceptors "github.com/panagiotisptr/proxygen/interceptor"

    {{range $import := .Imports}}
    {{ if $import.Used }} {{ $import.Alias }} "{{ $import.Path }}" {{ end }}
    {{- end }}
)

type {{ .Name }} struct {
	Implementation {{ .ImplementationType }}
	Interceptors   proxygenInterceptors.InterceptorChain
}

var _ {{ .ImplementationType }} = (*{{ .Name }})(nil)

{{- range $method := .Methods }}

func (this *{{ $.Name }}) {{ $method.Name }}(
{{- if gt (len $method.Params) 0 -}}
{{range $idx, $param := $method.Params }}
   arg{{ $idx }} {{ $param }},
{{- end}}
{{end -}}
) {{ if ne (len $method.Rets) 0 }}(
{{- range $ret := $method.Rets }}
   {{ $ret }},
{{- end}}
) {{end}}{
    {{if ne (len $method.Rets) 0 -}}
    rets := this.Interceptors.Apply(
    {{- else -}}
    this.Interceptors.Apply(
    {{- end}}
        []interface{}{
        {{- if gt (len $method.Params) 0 -}}
        {{range $idx, $param := $method.Params }}
           arg{{ $idx }},
        {{- end}}
        {{end -}}
        },
        "{{ $method.Name }}",
        func(args []interface{}) []interface{} {
            {{if ne (len $method.Rets) 0 -}}
            {{range $idx, $ret := $method.Rets -}}
            {{- if ne $idx 0 -}}
            ,
            res{{ $idx }}
            {{- else -}}
            res{{ $idx }}
            {{- end }}
            {{- end}} := this.Implementation.{{ $method.Name }}(
                {{- if gt (len $method.Params) 0 -}}
                {{range $idx, $param := $method.Params }}
                   args[{{ $idx }}].({{ $param }}),
                {{- end}}
                {{end -}}
            )
            {{- else -}}
            this.Implementation.{{ $method.Name }}(
                {{- if gt (len $method.Params) 0 -}}
                {{range $idx, $param := $method.Params }}
                   args[{{ $idx }}].({{ $param }}),
                {{- end}}
                {{end -}}
            )
            {{- end}}
        {{if eq (len $method.Rets) 0}}
            return []interface{}{}
        {{- else}}
            return []interface{}{
            {{- range $idx, $ret := $method.Rets }}
                res{{ $idx }},
            {{- end}}
            }
        {{- end}}
        },
    )

    {{if ne (len $method.Rets) 0 -}}
    return {{range $idx, $ret := $method.Rets -}}
        {{- if ne $idx 0 -}}
        ,
        rets[{{ $idx }}].({{ $ret }})
        {{- else -}}
        rets[{{ $idx }}].({{ $ret }})
        {{- end -}} 
    {{end}}
    {{- end}}
}
{{- end}}`
