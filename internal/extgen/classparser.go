package extgen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

var phpClassRegex = regexp.MustCompile(`//\s*export_php:?\s*class\s+(\w+)`)

type ExportDirective struct {
	Line      int
	ClassName string
}

type ClassParser struct{}

func (cp *ClassParser) parse(filename string) ([]PHPClass, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing file: %w", err)
	}

	var classes []PHPClass
	validator := NewValidator()

	exportDirectives := cp.collectExportDirectives(node, fset)

	// match structs to directives
	matchedDirectives := make(map[int]bool)

	var genDecl *ast.GenDecl
	var ok bool
	for _, decl := range node.Decls {
		if genDecl, ok = decl.(*ast.GenDecl); !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			var typeSpec *ast.TypeSpec
			if typeSpec, ok = spec.(*ast.TypeSpec); !ok {
				continue
			}

			var structType *ast.StructType
			if structType, ok = typeSpec.Type.(*ast.StructType); !ok {
				continue
			}

			var phpClass string
			var directiveLine int
			if phpClass, directiveLine = cp.extractPHPClassCommentWithLine(genDecl.Doc, fset); phpClass == "" {
				continue
			}

			matchedDirectives[directiveLine] = true

			class := PHPClass{
				Name:     phpClass,
				GoStruct: typeSpec.Name.Name,
			}

			class.Properties = cp.parseStructFields(structType.Fields.List)

			if err := validator.ValidateClass(class); err != nil {
				fmt.Printf("Warning: Invalid class '%s': %v\n", class.Name, err)
				continue
			}

			classes = append(classes, class)
		}
	}

	for _, directive := range exportDirectives {
		if !matchedDirectives[directive.Line] {
			return nil, fmt.Errorf("//export_php class directive at line %d is not followed by a struct declaration", directive.Line)
		}
	}

	return classes, nil
}

func (cp *ClassParser) collectExportDirectives(node *ast.File, fset *token.FileSet) []ExportDirective {
	var directives []ExportDirective

	for _, commentGroup := range node.Comments {
		for _, comment := range commentGroup.List {
			if matches := phpClassRegex.FindStringSubmatch(comment.Text); matches != nil {
				pos := fset.Position(comment.Pos())
				directives = append(directives, ExportDirective{
					Line:      pos.Line,
					ClassName: matches[1],
				})
			}
		}
	}

	return directives
}

func (cp *ClassParser) extractPHPClassCommentWithLine(commentGroup *ast.CommentGroup, fset *token.FileSet) (string, int) {
	if commentGroup == nil {
		return "", 0
	}

	for _, comment := range commentGroup.List {
		if matches := phpClassRegex.FindStringSubmatch(comment.Text); matches != nil {
			pos := fset.Position(comment.Pos())
			return matches[1], pos.Line
		}
	}

	return "", 0
}

func (cp *ClassParser) extractPHPClassComment(commentGroup *ast.CommentGroup) string {
	if commentGroup == nil {
		return ""
	}

	for _, comment := range commentGroup.List {
		if matches := phpClassRegex.FindStringSubmatch(comment.Text); matches != nil {
			return matches[1]
		}
	}

	return ""
}

func (cp *ClassParser) parseStructFields(fields []*ast.Field) []ClassProperty {
	var properties []ClassProperty

	for _, field := range fields {
		for _, name := range field.Names {
			prop := cp.parseStructField(name.Name, field)
			properties = append(properties, prop)
		}
	}

	return properties
}

func (cp *ClassParser) parseStructField(fieldName string, field *ast.Field) ClassProperty {
	prop := ClassProperty{Name: fieldName}

	// check if field is a pointer (nullable)
	if starExpr, isPointer := field.Type.(*ast.StarExpr); isPointer {
		prop.IsNullable = true
		prop.GoType = cp.typeToString(starExpr.X)
	} else {
		prop.IsNullable = false
		prop.GoType = cp.typeToString(field.Type)
	}

	prop.Type = cp.goTypeToPHPType(prop.GoType)
	return prop
}

func (cp *ClassParser) typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + cp.typeToString(t.X)
	case *ast.ArrayType:
		return "[]" + cp.typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + cp.typeToString(t.Key) + "]" + cp.typeToString(t.Value)
	default:
		return "interface{}"
	}
}

func (cp *ClassParser) goTypeToPHPType(goType string) string {
	goType = strings.TrimPrefix(goType, "*")

	typeMap := map[string]string{
		"string": "string",
		"int":    "int", "int64": "int", "int32": "int", "int16": "int", "int8": "int",
		"uint": "int", "uint64": "int", "uint32": "int", "uint16": "int", "uint8": "int",
		"float64": "float", "float32": "float",
		"bool": "bool",
	}

	if phpType, exists := typeMap[goType]; exists {
		return phpType
	}

	if strings.HasPrefix(goType, "[]") || strings.HasPrefix(goType, "map[") {
		return "array"
	}

	return "mixed"
}
