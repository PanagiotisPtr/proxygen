package generate

import (
	"go/ast"
	"go/token"
	"go/types"
	"os"

	"golang.org/x/tools/go/packages"
)

type TypeProcessor struct {
	defs    map[string]types.Object
	fset    *token.FileSet
	content []byte
}

func NewTypeProcessor(
	pkg *packages.Package,
	fset *token.FileSet,
	interfaceFile string,
) (*TypeProcessor, error) {
	defs := make(map[string]types.Object)

	for _, t := range pkg.TypesInfo.Defs {
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
		defs[t.Name()] = t
	}
	content, err := os.ReadFile(interfaceFile)
	if err != nil {
		return nil, err
	}

	return &TypeProcessor{
		defs:    defs,
		fset:    fset,
		content: content,
	}, nil
}

func (tp *TypeProcessor) deepIdent(t ast.Expr) *ast.Ident {
	switch t := t.(type) {
	case *ast.Ident:
		return t
	case *ast.StarExpr:
		return tp.deepIdent(t.X)
	case *ast.SelectorExpr:
		return tp.deepIdent(t.Sel)
	case *ast.ArrayType:
		return tp.deepIdent(t.Elt)
	case *ast.MapType:
		return tp.deepIdent(t.Value)
	case *ast.ChanType:
		return tp.deepIdent(t.Value)
	default:
		return nil
	}
}

func (tp *TypeProcessor) correctType(
	t ast.Expr,
	existingImports []*ImportData,
	newImports []*ImportData,
	pkgPath string,
) string {
	correctTypeProxy := func(t ast.Expr) string {
		return tp.correctType(t, existingImports, newImports, pkgPath)
	}

	switch t := t.(type) {
	case *ast.Ident:
		for _, defs := range tp.defs {
			if defs.Name() == t.Name {
				for i := range newImports {
					if newImports[i].Path == pkgPath {
						return newImports[i].Alias + "." + t.Name
					}
				}
			}
		}
		return t.Name
	case *ast.StarExpr:
		return "*" + correctTypeProxy(t.X)
	case *ast.SelectorExpr:
		selectorStart := tp.fset.Position(t.X.Pos())
		selectorEnd := tp.fset.Position(t.X.End())
		selectorPkg := string(tp.content[selectorStart.Offset:selectorEnd.Offset])
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
		return selectorPkg + "." + tp.deepIdent(t.Sel).Name
	case *ast.ArrayType:
		return "[]" + correctTypeProxy(t.Elt)
	case *ast.MapType:
		return "map[" + correctTypeProxy(t.Key) + "]" + correctTypeProxy(t.Value)
	case *ast.ChanType:
		// channel with correct arrow position
		if t.Arrow == token.NoPos {
			return "chan " + correctTypeProxy(t.Value)
		} else if t.Dir == ast.RECV {
			return "<-chan " + correctTypeProxy(t.Value)
		} else {
			return "chan<- " + correctTypeProxy(t.Value)
		}
	default:
		tokenStart := tp.fset.Position(t.Pos())
		tokenEnd := tp.fset.Position(t.End())
		return string(tp.content[tokenStart.Offset:tokenEnd.Offset])
	}
}
