package main

import (
	"os"
	"path"
	"flag"

	"fmt"
	"strings"
	"regexp"

	"container/vector"

	"go/parser"
	"go/token"
	"go/ast"
)

import "reflect"

func Typeof(o interface{}) string {
	if o == nil {
		return "<nil>"
	}
	return reflect.Typeof(o).String()
}

type Declaration struct {
	public      bool
	kind        int
	match, full string
}

type Declarations struct {
	file_name, package_name string
	decls                   []*Declaration
}

const (
	Import = 1 << iota
	Func
	Global
	Type
)

var (
	d = flag.Bool("d", false, "Show private declarations")
	D = flag.Bool("D", false, "Show public declarations")

	i = flag.Bool("i", false, "Show import declarations")
	f = flag.Bool("f", false, "Show function/method declarations")
	g = flag.Bool("g", false, "Show global (var and const) declarations")
	t = flag.Bool("t", false, "Show type declarations")

	m = flag.String("m", ".*", "Match declarations against pattern")

	p     = flag.Bool("p", false, "Hide file/package names")
	q     = flag.Bool("q", false, "Hide error messages")
	tests = flag.Bool("tests", false, "Include *_test.go files")
)


/*** Helper functions ***/

func check(e os.Error) {
	if e != nil {
		os.Stderr.WriteString(e.String() + "\n")
		os.Exit(1)
	}
}

func join(sv *vector.StringVector, sep string) string {
	return strings.Join(sv.Data(), sep)
}


/*** Gathering data ***/

func FieldList(fl *ast.FieldList) *vector.StringVector {
	vec := new(vector.StringVector)
	if fl == nil {
		vec.Push("")
		return vec
	}
	for _, field := range fl.List {
		if el, ok := field.Type.(*ast.Ellipsis); ok {
			vec.Push("..." + TypeExpr(el.Elt))
		} else {
			if len(field.Names) == 0 {
				vec.Push(TypeExpr(field.Type))
			} else {
				for i := 0; i < len(field.Names); i++ {
					vec.Push(TypeExpr(field.Type))
				}
			}
		}
	}
	return vec
}

func FuncBack(parm, rets *ast.FieldList) string {
	r, p := " ", join(FieldList(parm), ", ")
	retvec := FieldList(rets)
	jrets := join(retvec, ", ")
	switch retvec.Len() {
	case 0:
	case 1:
		r += jrets
	default:
		r += "(" + jrets + ")"
	}
	return fmt.Sprintf("(%s)%s", p, r)
}

func typeExpr(E ast.Expr, vec *vector.StringVector) *vector.StringVector {
	switch e := E.(type) {
	case *ast.ArrayType:
		vec.Push("[")
		if _, ok := e.Len.(*ast.Ellipsis); ok {
			vec.Push("...")
		} else if e.Len != nil {
			vec.Push(string(e.Len.(*ast.BasicLit).Value))
		}
		vec.Push("]")
		typeExpr(e.Elt, vec)

	case *ast.MapType:
		vec.Push("map[")
		typeExpr(e.Key, vec)
		vec.Push("]")
		typeExpr(e.Value, vec)

	case *ast.ChanType:
		if e.Dir == ast.RECV {
			vec.Push("<-")
		}
		vec.Push("chan")
		if e.Dir == ast.SEND {
			vec.Push("<- ")
		} else {
			vec.Push(" ")
		}
		typeExpr(e.Value, vec)

	case *ast.StarExpr:
		vec.Push("*")
		typeExpr(e.X, vec)

	case *ast.SelectorExpr:
		typeExpr(e.X, vec)
		vec.Push(".")
		typeExpr(e.Sel, vec)

	case *ast.Ident:
		vec.Push(e.String())

	case *ast.FuncType:
		vec.Push("func")
		vec.Push(FuncBack(e.Params, e.Results))

	case *ast.StructType:
		vec.Push("struct {")
		for _, field := range e.Fields.List {
			vec.Push("\n\t")
			//name_type(false, field.Names, field.Type, vec)
			names := field.Names
			switch ln := len(names); ln {
			case 0:
			case 1:
				vec.Push(names[0].String())
				vec.Push(" ")
			default:
				for _, nm := range names[0 : ln-1] {
					vec.Push(nm.String())
					vec.Push(", ")
				}
				vec.Push(names[ln-1].String())
				vec.Push(" ")
			}
			typeExpr(field.Type, vec)
		}
		if len(e.Fields.List) > 0 {
			vec.Push("\n")
		}
		vec.Push("}")

	case *ast.InterfaceType:
		vec.Push("interface {")
		if len(e.Methods.List) != 0 {
			for _, method := range e.Methods.List {
				vec.Push("\n\t")
				if len(method.Names) != 0 {
					vec.Push(method.Names[0].String())
					mt := method.Type.(*ast.FuncType)
					vec.Push(FuncBack(mt.Params, mt.Results))
				} else {
					vec.Push(method.Type.(*ast.Ident).String())
				}
			}
			vec.Push("\n")
		}
		vec.Push("}")
	}
	return vec
}

func TypeExpr(E ast.Expr) string {
	v := new(vector.StringVector)
	typeExpr(E, v)
	return join(v, "")
}


/*** the four top level declarations ***/

func TypeDecl(decl *ast.GenDecl, vec *vector.Vector) {
	for _, spec := range decl.Specs {
		ts := spec.(*ast.TypeSpec)
		name := ts.Name.String()
		vec.Push(&Declaration{
			ts.Name.IsExported(),
			Type,
			name,
			fmt.Sprintf("type %s %s", name, TypeExpr(ts.Type))})
	}
}

func FuncDecl(decl *ast.FuncDecl) *Declaration {
	recvr := " "
	if decl.Recv != nil {
		R := decl.Recv.List[0].Type
		recvr = " (" + TypeExpr(R) + ") "
	}
	name := decl.Name.String()
	back := FuncBack(decl.Type.Params, decl.Type.Results)
	return &Declaration{
		decl.Name.IsExported(),
		Func,
		name,
		fmt.Sprintf("func%s%s%s", recvr, name, back)}
}

func ImportDecl(decl *ast.GenDecl, vec *vector.Vector) {
	for _, spec := range decl.Specs {
		is := spec.(*ast.ImportSpec)
		local := ""
		if is.Name != nil {
			local = is.Name.String() + " "
		}
		path := string(is.Path.Value)
		vec.Push(&Declaration{
			true, //we just ignore it
			Import,
			path,
			fmt.Sprintf(`import %s%s`, local, path)})
	}
}

func GlobalDecl(decl *ast.GenDecl, kind string, vec *vector.Vector) {
	var last_type, cur_type string
	for _, spec := range decl.Specs {
		vs := spec.(*ast.ValueSpec)
		if vs.Type != nil {
			last_type = TypeExpr(vs.Type)
		}
		cur_type = last_type
		for _, name := range vs.Names {
			vec.Push(&Declaration{
				name.IsExported(),
				Global,
				name.String(),
				fmt.Sprintf("%s %s %s", kind, name.String(), cur_type)})
		}
	}
}

func doFile(f string) *Declarations {
	out, decvec := &Declarations{file_name: f}, new(vector.Vector)
	file, err := parser.ParseFile(f, nil, nil, 0)
	check(err)

	out.package_name = file.Name.String()

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			switch d.Tok {
			case token.IMPORT:
				ImportDecl(d, decvec)
			case token.TYPE:
				TypeDecl(d, decvec)
			case token.CONST:
				GlobalDecl(d, "const", decvec)
			case token.VAR:
				GlobalDecl(d, "var", decvec)
			}
		case *ast.FuncDecl:
			decvec.Push(FuncDecl(d))
		case *ast.BadDecl:
			if !*q {
				os.Stderr.WriteString(
					"Bad declarations in " + f + " Results may be ommitted.")
			}
		}
	}

	out.decls = make([]*Declaration, decvec.Len())
	count := 0
	for decl := range decvec.Iter() {
		out.decls[count] = decl.(*Declaration)
		count++
	}

	return out
}

func main() {
	/* handle all the flags */
	flag.Parse()
	if flag.NArg() == 0 {
		if !*q {
			println("No files specified")
		}
		os.Exit(1)
	}

	//if neither are set, show both
	if !*d && !*D {
		*d, *D = true, true
	}
	//if none are set, show all
	if !*i && !*f && !*g && !*t {
		*i, *f, *g, *t = true, true, true, true
	}

	var filter int
	if *i {
		filter |= Import
	}
	if *f {
		filter |= Func
	}
	if *g {
		filter |= Global
	}
	if *t {
		filter |= Type
	}

	matcher, err := regexp.Compile(*m)
	check(err)

	files := flag.Args()

	/* process all the files */

	var glob vector.Vector
	for _, file_name := range files {
		info, err := os.Stat(file_name)
		check(err)

		if info.IsDirectory() {
			base := path.Clean(file_name)

			dir, err := os.Open(base, os.O_RDONLY, 0664)
			defer dir.Close()
			check(err)

			dirs, err := dir.Readdirnames(-1)
			check(err)

			none := true
			for _, file_name = range dirs {
				if !*tests && strings.HasSuffix(file_name, "_test.go") {
					continue
				}
				if strings.HasSuffix(file_name, ".go") {
					none = false
					glob.Push(doFile(path.Join(base, file_name)))
				}
			}
			if none {
				if !*q {
					os.Stderr.WriteString("There are no go files in " + base)
				}
				os.Exit(1)
			}
		} else if info.IsRegular() {
			//file.Close()
			glob.Push(doFile(file_name))
		} else {
			msg := "File " + file_name
			msg += " is neither a directory nor a regular file"
			if !*q {
				os.Stderr.WriteString(msg)
			}
			os.Exit(1)
		}
	}

	/* display all requested results */

	var show bool
	for e := range glob.Iter() {
		entry := e.(*Declarations)
		if !*p {
			println(entry.file_name + ":" + entry.package_name)
		}
		for _, decls := range entry.decls {
			show = filter&decls.kind != 0
			if show && decls.kind != Import {
				show = (*D && decls.public) || (*d && !decls.public)
			}
			if show && matcher.MatchString(decls.match) {
				println(decls.full)
			}
		}
	}
}
