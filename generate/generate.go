package generate

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"html/template"
	"os"
	"reflect"

	"golang.org/x/tools/go/packages"
)

type MethodData struct {
	Name   string
	Params []string
	Rets   []string
}

type InterfaceData struct {
	InterfacePackage string
	InterfaceName    string
	Imports          []string
	Methods          []MethodData
}

const mode packages.LoadMode = packages.NeedName |
	packages.NeedTypes |
	packages.NeedSyntax |
	packages.NeedTypesInfo

func LoadPackage() {
	packagePath := "github.com/panagiotisptr/service-proxies-demo/service"
	interfaceName := "SomeService"

	fmt.Println(packagePath)
	fmt.Println(interfaceName)

	var fset = token.NewFileSet()
	cfg := &packages.Config{Fset: fset, Mode: mode, Dir: "."}

	data, err := getInterfaceData(fset, cfg, packagePath, interfaceName)
	if err != nil {
		fmt.Println("err: ", err)
		os.Exit(1)
	}

	return
	//render template
	tmpl := template.Must(template.New("proxy").Parse(t))
	var buf bytes.Buffer
	er := tmpl.Execute(&buf, struct {
		PackageName string
		Name        string
		InterfaceData
	}{
		PackageName:   "test",
		Name:          "test",
		InterfaceData: data,
	})

	generatedProxy := buf
	fmt.Println("buf: ", generatedProxy.String())
	fmt.Println("err: ", er)

	// foramt generated proxy
	bs, e := format.Source(generatedProxy.Bytes())
	fmt.Println("formatted: ", string(bs))
	fmt.Println("err: ", e)
}

func getInterfaceData(
	fset *token.FileSet,
	cfg *packages.Config,
	interfacePackage string,
	interfaceName string,
) (InterfaceData, error) {
	data := InterfaceData{
		InterfacePackage: interfacePackage,
		InterfaceName:    interfaceName,
	}
	pkgs, err := packages.Load(cfg, interfacePackage)
	if err != nil {
		return data, err
	}

	uses := make(map[string]types.Object)
	var pkg *packages.Package
	for _, p := range pkgs {
		if p.PkgPath == interfacePackage {
			pkg = p
			// print all types in package
			for _, t := range p.TypesInfo.Uses {
				if t == nil {
					continue
				}
				if !t.Exported() {
					continue
				}
				if _, ok := uses[t.Name()]; ok {
					continue
				}
				fmt.Println("USES:", t.Name(), "from package:", t.Pkg().Path())
				uses[t.Name()] = t
			}
		}
	}
	var iface *ast.InterfaceType
	var f *ast.File

	for _, fileAst := range pkg.Syntax {
		f = fileAst
		ast.Inspect(fileAst, func(n ast.Node) bool {
			switch t := n.(type) {
			case *ast.TypeSpec:
				if !t.Name.IsExported() {
					return false
				}
				if t.Name.Name != interfaceName {
					return false
				}
				fmt.Println("typeSpec: ", t.Name.Name)
				switch ti := t.Type.(type) {
				case *ast.InterfaceType:
					iface = ti
					return false
				default:
					return false
				}
			}

			return true
		})

		filename := fset.Position(f.Package).Filename
		fmt.Println("filepath: ", filename)
		content, _ := os.ReadFile(filename)
		start := fset.Position(iface.Pos())
		end := fset.Position(iface.End())
		fmt.Println("interface: ", string(content[start.Offset:end.Offset]))

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

			if m.Type != nil {
				switch t := m.Type.(type) {
				case *ast.FuncType:
					if t.Params != nil {
						for _, param := range t.Params.List {
							start = fset.Position(param.Type.Pos())
							end = fset.Position(param.Type.End())
							methodData.Params = append(
								methodData.Params,
								string(content[start.Offset:end.Offset]),
							)
							rt := realType(param.Type)
							fmt.Println("param: ", param.Names, "- ", reflect.TypeOf(param.Type), string(content[start.Offset:end.Offset]), " - realtype: ", rt)
							if rt.IsExported() {
								fmt.Println("exported: ", rt.Name)
							}
						}
					}

					if t.Results != nil {
						for _, result := range t.Results.List {
							start = fset.Position(result.Pos())
							end = fset.Position(result.End())
							methodData.Rets = append(
								methodData.Rets,
								string(content[start.Offset:end.Offset]),
							)
							fmt.Println("result: ", result.Names, "- ", reflect.TypeOf(result.Type), string(content[start.Offset:end.Offset]), " - realtype: ", realType(result.Type))
						}
					}
				}
			}

			data.Methods = append(data.Methods, methodData)
		}
	}

	return data, nil
}

const t = `// Code generated by proxygen. DO NOT EDIT.
package {{ .PackageName }}

import (
    proxygenInterceptors "github.com/panagiotisptr/proxygen/interceptor"

    {{range $idx, $import := .Imports}}
    {{ import{{$idx}} $import }}
    {{- end }}
)

type {{ .Name }} struct {
	Implementation iservice.TaskService
	Interceptors   proxygenInterceptors.InterceptorChain
}

{{ range $method := .Methods }}

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
    rets := this.Interceptors.Apply(
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
                   arg{{ $idx }}.({{ $param }}),
                {{- end}}
                {{end -}}
            )
            {{- else -}}
            this.Implementation.{{ $method.Name }}(
                {{- if gt (len $method.Params) 0 -}}
                {{range $idx, $param := $method.Params }}
                   arg{{ $idx }}.({{ $param }}),
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
