package extgen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

type ClassParser struct{}

func (cp *ClassParser) parse(filename string) ([]PHPClass, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing file: %w", err)
	}

	var classes []PHPClass
	validator := NewValidator()

	for _, decl := range node.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if structType, ok := typeSpec.Type.(*ast.StructType); ok {
						if phpClass := cp.extractPHPClassComment(genDecl.Doc); phpClass != "" {
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
				}
			}
		}
	}

	return classes, nil
}

func (cp *ClassParser) extractPHPClassComment(commentGroup *ast.CommentGroup) string {
	if commentGroup == nil {
		return ""
	}

	phpClassRegex := regexp.MustCompile(`//\s*php_class:\s*(\w+)`)

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
