package macro

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// DeriveDirective is the comment prefix for derive directives.
const DeriveDirective = "gpp:derive"

// ParseDeriveDirective extracts derive targets from a comment line.
// "//gpp:derive String,Validate" → ["String", "Validate"]
func ParseDeriveDirective(commentText string) []string {
	text := strings.TrimSpace(commentText)
	text = strings.TrimPrefix(text, "//")
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, DeriveDirective) {
		return nil
	}
	text = strings.TrimPrefix(text, DeriveDirective)
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	parts := strings.Split(text, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// FindConstNamesForType searches all const declarations in a file
// for constants of the given type name. Returns the constant names.
func FindConstNamesForType(file *ast.File, typeName string) []string {
	var names []string
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}
		currentType := ""
		for _, spec := range genDecl.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			// Update current type from explicit type annotation
			if vs.Type != nil {
				if ident, ok := vs.Type.(*ast.Ident); ok {
					currentType = ident.Name
				}
			}
			// Collect names if they match the target type
			if currentType == typeName {
				for _, name := range vs.Names {
					names = append(names, name.Name)
				}
			}
		}
	}
	return names
}

// GenerateStringMethod creates a String() method AST for the given type
// with a switch statement over the constant names.
func GenerateStringMethod(typeName string, constNames []string) *ast.FuncDecl {
	// Pick a short receiver name (lowercase first char)
	receiverName := strings.ToLower(typeName[:1])
	if len(typeName) > 1 && receiverName == "_" {
		receiverName = strings.ToLower(typeName[1:2])
	}

	// Build case clauses
	var caseClauses []ast.Stmt
	for _, name := range constNames {
		caseClauses = append(caseClauses, &ast.CaseClause{
			List: []ast.Expr{ast.NewIdent(name)},
			Body: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.BasicLit{
							Kind:  token.STRING,
							Value: fmt.Sprintf("\"%s\"", name),
						},
					},
				},
			},
		})
	}

	// Default case: fmt.Sprintf("TypeName(%d)", int(receiver))
	defaultClause := &ast.CaseClause{
		List: nil, // default
		Body: []ast.Stmt{
			&ast.ReturnStmt{
				Results: []ast.Expr{
					&ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   ast.NewIdent("fmt"),
							Sel: ast.NewIdent("Sprintf"),
						},
						Args: []ast.Expr{
							&ast.BasicLit{
								Kind:  token.STRING,
								Value: fmt.Sprintf("\"%s(%%d)\"", typeName),
							},
							&ast.CallExpr{
								Fun:  ast.NewIdent("int"),
								Args: []ast.Expr{ast.NewIdent(receiverName)},
							},
						},
					},
				},
			},
		},
	}
	caseClauses = append(caseClauses, defaultClause)

	// Switch statement
	switchStmt := &ast.SwitchStmt{
		Tag: ast.NewIdent(receiverName),
		Body: &ast.BlockStmt{
			List: caseClauses,
		},
	}

	return &ast.FuncDecl{
		Recv: &ast.FieldList{List: []*ast.Field{{
			Names: []*ast.Ident{ast.NewIdent(receiverName)},
			Type:  ast.NewIdent(typeName),
		}}},
		Name: ast.NewIdent("String"),
		Type: &ast.FuncType{
			Params:  &ast.FieldList{},
			Results: &ast.FieldList{List: []*ast.Field{{Type: ast.NewIdent("string")}}},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{switchStmt},
		},
	}
}

// ProcessDeriveDirectives scans a file for //gpp:derive comments on type
// declarations and generates the requested methods. Returns the list of
// generated method declarations.
func ProcessDeriveDirectives(file *ast.File) []*ast.FuncDecl {
	var generated []*ast.FuncDecl

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		if genDecl.Doc == nil {
			continue
		}

		// Check for derive directive in doc comments
		var derives []string
		for _, comment := range genDecl.Doc.List {
			d := ParseDeriveDirective(comment.Text)
			if len(d) > 0 {
				derives = append(derives, d...)
			}
		}
		if len(derives) == 0 {
			continue
		}

		// Process each type spec in the declaration
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			typeName := typeSpec.Name.Name

			for _, d := range derives {
				switch d {
				case "String":
					// Find const names for this type
					constNames := FindConstNamesForType(file, typeName)
					if len(constNames) == 0 {
						continue
					}
					method := GenerateStringMethod(typeName, constNames)
					generated = append(generated, method)
				}
			}
		}
	}

	return generated
}
