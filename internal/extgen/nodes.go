package extgen

type PHPFunction struct {
	Name             string
	Signature        string
	GoFunction       string
	Params           []Parameter
	ReturnType       string
	IsReturnNullable bool
}

type Parameter struct {
	Name         string
	Type         string
	IsNullable   bool
	DefaultValue string
	HasDefault   bool
}

type PHPClass struct {
	Name       string
	GoStruct   string
	Properties []ClassProperty
}

type ClassProperty struct {
	Name       string
	Type       string
	GoType     string
	IsNullable bool
}
