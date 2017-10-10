package generator

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"unicode"

	graphql "github.com/neelance/graphql-go"
	"github.com/neelance/graphql-go/introspection"
)

const (
	gqlSCALAR       = "SCALAR"
	gqlINTERFACE    = "INTERFACE"
	gqlINPUT_OBJECT = "INPUT_OBJECT"
	gqlUNION        = "UNION"
	gqlLIST         = "LIST"
	gqlOBJECT       = "OBJECT"
	gqlENUM         = "ENUM"
	gqlDIRECTIVE    = "DIRECTIVE"
	gqlID           = "ID"
)

var KnownGQLTypes = map[string]bool{
	"__Directive":         true,
	"__DirectiveLocation": true,
	"__EnumValue":         true,
	"__Field":             true,
	"__InputValue":        true,
	"__Schema":            true,
	"__Type":              true,
	"__TypeKind":          true,
	"Query":               true,
	"Mutation":            true,
	"LIST":                true,
	"ENUM":                true,
	"INTERFACE":           true,
	"String":              true,
	"Float":               true,
	"ID":                  true,
	"Int":                 true,
	"Boolean":             true,
	"Time":                true,
}

var KnownGoTypes = map[string]bool{
	"string":  true,
	"bool":    true,
	"float32": true,
	"float64": true,
	"int":     true,
	"int32":   true,
	"int64":   true,
}

type TypeDef struct {
	Name        string
	Description string
	Fields      map[string]*FieldDef
	GQLType     string
	gqlType     *introspection.Type
}

func NewType(t *introspection.Type) (typ *TypeDef) {
	tp := &TypeDef{
		Name:        pts(t.Name()),
		Description: pts(t.Description()),
		Fields:      map[string]*FieldDef{},
		gqlType:     t,
	}
	for _, fld := range *t.Fields(nil) {
		f := NewField(fld)
		f.Parent = tp.Name
		tp.Fields[f.Name] = f
	}
	return tp
}

type FieldDef struct {
	Name        string
	Parent      string
	Description string
	Type        *Typ
	gqlField    *introspection.Field
}

func NewField(t *introspection.Field) (fld *FieldDef) {
	name := upperFirst(t.Name())
	if name == "Id" {
		name = "ID"
	}
	fld = &FieldDef{
		Name:        name,
		Description: pts(t.Description()),
		Type: &Typ{
			IsNullable: true,
			gqlType:    t.Type(),
		},
	}
	fld.Parse()
	return fld
}

func (f *FieldDef) Parse() {

	tp := f.Type.gqlType
	td := f.Type

FindGoType:
	td.gqlType = tp
	if tp.Kind() == "NON_NULL" {
		td.IsNullable = false
		tp = tp.OfType()
	}

	if tp.Kind() == "LIST" {
		td.GoType = "[]"
		td.GQLType = "[]"

		td.Type = &Typ{
			IsNullable: true,
		}

		tp = tp.OfType()
		td = td.Type
		goto FindGoType
	}

	switch *tp.Name() {
	case "String":
		td.GoType = "string"
		td.GQLType = "string"
	case "Int":
		td.GoType = "int32"
		td.GQLType = "int32"
	case "Float":
		td.GoType = "float32"
		td.GQLType = "float32"
	case "ID":
		td.GoType = "string"
		td.GQLType = "graphql.ID"
	case "Boolean":
		td.GoType = "bool"
		td.GQLType = "bool"
	case "Time":
		td.GoType = "time.Time"
		td.GQLType = "graphql.Time"
	default:
		td.GoType = pts(tp.Name())
		td.GQLType = lowerFirst(pts(tp.Name())) + "Resolver"
	}
	return
}

type Typ struct {
	GoType     string
	GQLType    string
	IsNullable bool
	Type       *Typ
	Values     []string
	gqlType    *introspection.Type
}

func (t Typ) genType(mode string) string {
	var r string

	if mode == "struct" {
		r = t.GoType
		ok := KnownGoTypes[t.GoType]
		if t.IsNullable && t.GQLType != "[]" && !ok {
			r = "*" + r
		}
	} else {
		r = t.GQLType
		if t.IsNullable {
			r = "*" + r
		}
	}
	if t.Type == nil {
		return r
	}

	r = r + t.Type.genType(mode)
	return r
}

func (t *TypeDef) GenStruct() string {
	r := "type " + t.Name + " struct {\n"
	for _, fld := range t.Fields {
		r = r + "  " + fld.Name + " " + fld.Type.genType("struct") + "\n"
	}
	r = r + "}"
	return r
}

func (t *TypeDef) GenResStruct(typePkgName string) string {
	if typePkgName != "" {
		typePkgName = typePkgName + "."
	}

	r := "type " + lowerFirst(t.Name) + "Resolver struct {\n"
	r = r + "  r *" + t.Name + "\n"
	r = r + "}"
	return r
}

// Resolvers generates the resolver function for the given FieldDef
func (f FieldDef) GenResolver() string {
	res := f.Type.genType("resolver")
	returnType := res
	r := "func (r *" + lowerFirst(f.Parent) + "Resolver) " + f.Name + "() " + res + " {\n"
	itm := ""

	if f.Type.GQLType == "[]" {
		if f.Type.IsNullable {
			returnType = strings.Replace(returnType, "*", "", 1)
		}
		if _, ok := KnownGoTypes[f.Type.Type.GoType]; !ok {
			ref := ""
			dref := ""
			if f.Type.Type.IsNullable {
				itm = "&"
				dref = "*"
			} else {
				ref = "&"
			}

			if f.Type.Type.GQLType == "graphql.ID" {
				itm = f.Type.Type.GQLType + "(itm)"
			} else if f.Type.Type.GQLType == "graphql.Time" {
				itm = itm + f.Type.Type.GQLType + "{Time: " + dref + "itm}"
			} else {
				itm = itm + f.Type.Type.GQLType + "{r: " + ref + "itm}"
			}
		} else {

			if f.Type.Type.IsNullable {
				itm = "&itm"
			}
		}

		if itm == "" {
			itm = "itm"
		}
		r = r + "  items := " + returnType + "{}\n"
		r = r + "  for _, itm := range r.r." + f.Name + " {\n"
		r = r + "    items = append(items, " + itm + ")\n"
		r = r + "  }\n"
		r = r + "  return "
		if f.Type.IsNullable {
			r = r + "&"
		}
		r = r + "items\n"
		r = r + "}"
		return r
	}

	if f.Type.GQLType != "graphql.ID" {
		r = r + "  return "
		if f.Type.IsNullable {
			r = r + "&"
		}
	}

	if _, ok := KnownGoTypes[f.Type.GQLType]; !ok {
		if f.Type.GQLType == "graphql.ID" {
			r = r + "  id := graphql.ID(r.r." + f.Name + ")\n"
			r = r + "  return "
			if f.Type.IsNullable {
				r = r + "&"
			}
			r = r + "id"
		} else if f.Type.GQLType == "graphql.Time" {
			dref := ""
			if f.Type.IsNullable {
				dref = "*"
			}
			r = r + f.Type.GQLType + "{Time: " + dref + "r.r." + f.Name + "}"
		} else {
			ref := ""
			if !f.Type.IsNullable {
				ref = "&"
			}
			r = r + f.Type.GQLType + "{r: " + ref + "r.r." + f.Name + "}"
		}
	} else {
		r = r + "r.r." + f.Name
	}

	r = r + "\n}"
	return r
}

type Generator struct {
	*bytes.Buffer

	PkgName   string
	rawSchema []byte
	schema    *graphql.Schema

	//Param             map[string]string // Command-line parameters.
	//PackageImportPath string            // Go import path of the package we're generating code for
	//ImportPrefix      string            // String to prefix to imported package file names.
	//ImportMap         map[string]string // Mapping from .proto file name to import path

	//Pkg map[string]string // The names under which we import support packages

	//packageName string // What we're calling ourselves.
	//allFiles         []*FileDescriptor          // All files in the tree
	//allFilesByName   map[string]*FileDescriptor // All files by filename.
	//genFiles         []*FileDescriptor          // Those files we will generate output for.
	//file             *FileDescriptor            // The file we are compiling now.
	//usedPackages map[string]bool // Names of packages used in current file.
	//typeNameToObject map[string]Object          // Key is a fully-qualified name in input syntax.
	//init   []string // Lines to emit in the init function.
	indent string

	writeOutput bool
}

// P prints the arguments to the generated output.  It handles strings and int32s, plus
// handling indirections because they may be *string, etc.
func (g *Generator) P(str ...interface{}) {
	if !g.writeOutput {
		return
	}
	g.WriteString(g.indent)
	for _, v := range str {
		switch s := v.(type) {
		case string:
			g.WriteString(s)
		case *string:
			g.WriteString(*s)
		case bool:
			fmt.Fprintf(g, "%t", s)
		case *bool:
			fmt.Fprintf(g, "%t", *s)
		case int:
			fmt.Fprintf(g, "%d", s)
		case *int32:
			fmt.Fprintf(g, "%d", *s)
		case *int64:
			fmt.Fprintf(g, "%d", *s)
		case float64:
			fmt.Fprintf(g, "%g", s)
		case *float64:
			fmt.Fprintf(g, "%g", *s)
		default:
			g.Fail(fmt.Sprintf("unknown type in printer: %T", v))
		}
	}
	g.WriteByte('\n')
}

// New creates a new generator and allocates the request and response protobufs.
func New() *Generator {
	g := new(Generator)
	g.Buffer = new(bytes.Buffer)
	g.writeOutput = true
	//g.Request = new(plugin.CodeGeneratorRequest)
	//g.Response = new(plugin.CodeGeneratorResponse)
	return g
}

func (g *Generator) Parse(fileData []byte) error {
	g.rawSchema = fileData
	schema, err := graphql.ParseSchema(string(fileData), nil)
	g.schema = schema
	return err
}

func (g *Generator) SetPkgName(name string) *Generator {
	g.PkgName = name
	return g
}

// In Indents the output one tab stop.
func (g *Generator) In() { g.indent += "\t" }

// Out unindents the output one tab stop.
func (g *Generator) Out() {
	if len(g.indent) > 0 {
		g.indent = g.indent[1:]
	}
}

// Error reports a problem, including an error, and exits the program.
func (g *Generator) Error(err error, msgs ...string) {
	s := strings.Join(msgs, " ") + ":" + err.Error()
	log.Print("protoc-gen-go: error:", s)
	os.Exit(1)
}

// Fail reports a problem and exits the program.
func (g *Generator) Fail(msgs ...string) {
	s := strings.Join(msgs, " ")
	log.Print("protoc-gen-go: error:", s)
	os.Exit(1)
}

type SchemaMap struct {
	RootTypes     map[string]string
	ResolverTypes map[string]string
	ResolverFuncs map[string]string
}

func NewSchemaMap() *SchemaMap {
	return &SchemaMap{
		RootTypes:     map[string]string{},
		ResolverTypes: map[string]string{},
		ResolverFuncs: map[string]string{},
	}
}

// Fill the buffer with the generated output for all the files we're supposed to generate.
func (g Generator) GenSchemaResolversFile() ([]byte, []*TypeDef) {
	g.P("package ", g.PkgName)
	g.P("")
	g.P("import (")
	g.In()
	g.P(`"time"`)
	g.P(`graphql "github.com/neelance/graphql-go"`)
	g.Out()
	g.P(")")
	g.P("")

	types := []*TypeDef{}
	for _, typ := range g.schema.Inspect().Types() {
		if KnownGQLTypes[*typ.Name()] {
			continue
		}
		switch typ.Kind() {
		case gqlOBJECT:
			gtp := NewType(typ)

			g.P(gtp.GenStruct())
			g.P("")
			g.P(gtp.GenResStruct(""))
			g.P("")
			for _, f := range gtp.Fields {
				g.P(f.GenResolver())
				g.P("")
			}
			types = append(types, gtp)
		case gqlSCALAR:
			//TODO: Implement union type code generation
		case gqlINTERFACE:
			//TODO: Implement interface type code generation
		case gqlENUM:
			//TODO: Implement enum type code generation
		case gqlUNION:
			//TODO: Implement union type code generation
		case gqlINPUT_OBJECT:
			//TODO: Implement union type code generation
			//fmt.Println("Input Object ", *typ.Name())
		default:
			//TODO: Implement union type code generation
			//fmt.Println("unknown graphql type ", *typ.Name(), ":", typ.Kind())
		}
	}

	g.P("")
	g.P("var Schema = `")
	g.P(string(g.rawSchema))
	g.P("")
	g.P("`")
	return g.Bytes(), types
}

// Helper functions
func lowerFirst(s string) string {
	return firstCharToCase(s, "lower")
}

func upperFirst(s string) string {
	return firstCharToCase(s, "upper")
}

func firstCharToCase(s string, c string) string {
	done := false
	mapFunc := func(r rune) rune {
		if done || !unicode.IsLetter(r) {
			return r
		}
		done = true
		if c == "upper" {
			return unicode.ToUpper(r)
		}
		return unicode.ToLower(r)
	}
	return strings.Map(mapFunc, s)
}

func pts(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
