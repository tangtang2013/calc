// Copyright (c) 2014, Rob Thornton
// All rights reserved.
// This source code is governed by a Simplied BSD-License. Please see the
// LICENSE included in this distribution for a copy of the full license
// or, if one is not included, you may also find a copy at
// http://opensource.org/licenses/BSD-2-Clause

package comp

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/rthornton128/calc/ast"
	"github.com/rthornton128/calc/parse"
	"github.com/rthornton128/calc/token"
)

type compiler struct {
	fp       *os.File
	fset     *token.FileSet
	errors   token.ErrorList
	offset   int
	curScope *ast.Scope
	topScope *ast.Scope
}

func CompileFile(path string) {
	var c compiler

	c.fset = token.NewFileSet()
	f := parse.ParseFile(c.fset, path)
	if f == nil {
		os.Exit(1)
	}

	path = path[:len(path)-len(filepath.Ext(path))]
	fp, err := os.Create(path + ".c")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer fp.Close()

	c.fp = fp
	c.compFile(f)

	if c.errors.Count() != 0 {
		c.errors.Print()
		os.Exit(1)
	}
}

func CompileDir(path string) {
	fs := token.NewFileSet()
	pkg := parse.ParseDir(fs, path)
	if pkg == nil {
		os.Exit(1)
	}

	fp, err := os.Create(path + ".c")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer fp.Close()

	c := &compiler{fp: fp, fset: fs}
	c.compPackage(pkg)

	if c.errors.Count() != 0 {
		c.errors.Print()
		os.Exit(1)
	}
}

/* Utility */

func (c *compiler) Error(pos token.Pos, args ...interface{}) {
	c.errors.Add(c.fset.Position(pos), args...)
}

func roundUp16(n int) int {
	if r := n % 16; r != 0 {
		return n + (16 - r)
	}
	return n
}

func (c *compiler) nextOffset() (offset int) {
	offset = c.offset
	c.offset += 4
	return
}

/* Scope */

func (c *compiler) openScope(s *ast.Scope) {
	c.curScope = s
}

func (c *compiler) closeScope() {
	c.curScope = c.curScope.Parent
}

/* Main Compiler */

func (c *compiler) compNode(node ast.Node) int {
	switch n := node.(type) {
	case *ast.AssignExpr:
		c.compAssignExpr(n)
	case *ast.BasicLit:
		c.compInt(n, "eax")
	case *ast.BinaryExpr:
		return c.compBinaryExpr(n)
	case *ast.CallExpr:
		return c.compCallExpr(n)
	case *ast.DeclExpr:
		return c.compDeclExpr(n)
	case *ast.ExprList:
		for i := range n.List {
			c.compNode(n.List[i])
		}
	case *ast.Ident:
		c.compIdent(n, "movl(ebp+%d, eax);\n")
	case *ast.IfExpr:
		c.compIfExpr(n)
	case *ast.VarExpr:
		c.compVarExpr(n)
	}
	return 0
}

func (c *compiler) compAssignExpr(a *ast.AssignExpr) {
	ob := c.curScope.Lookup(a.Name.Name)
	if ob == nil {
		c.Error(a.Name.NamePos, "can't assign value to undeclared variable '",
			a.Name.Name, "'")
		return
	}
	atype, otype := typeOf(a.Value, c.curScope), typeOfObject(ob)
	if otype == "unknown" {
		ob.Type = &ast.Ident{NamePos: token.NoPos, Name: atype}
		otype = atype
	}
	if atype != otype {
		c.Error(a.Name.NamePos, "type mismatch, can't assign a value of type ",
			atype, " to a variable of type ", otype)
	}
	ob.Value = a.Value
	switch n := ob.Value.(type) {
	case *ast.BasicLit:
		c.compInt(n, fmt.Sprintf("ebp+%d", ob.Offset))
	case *ast.BinaryExpr:
		c.compBinaryExpr(n)
		fmt.Fprintf(c.fp, "movl(eax, ebp+%d);\n", ob.Offset)
	case *ast.CallExpr:
		c.compCallExpr(n)
		fmt.Fprintf(c.fp, "movl(eax, ebp+%d);\n", ob.Offset)
	case *ast.Ident:
		c.compIdent(n, fmt.Sprintf("movl(ebp+%%d, ebp+%d);\n", ob.Offset))
	}
}

func (c *compiler) compBinaryExpr(b *ast.BinaryExpr) int {
	switch n := b.List[0].(type) {
	case *ast.BasicLit:
		c.compInt(n, "eax")
	case *ast.BinaryExpr:
		c.compBinaryExpr(n)
	case *ast.CallExpr:
		c.compCallExpr(n)
	case *ast.Ident:
		c.compIdent(n, "movl(ebp+%d, eax);\n")
	}

	for _, node := range b.List[1:] {
		switch n := node.(type) {
		case *ast.BasicLit:
			c.compInt(n, "edx")
		case *ast.BinaryExpr:
			fmt.Fprintln(c.fp, "pushl(eax);")
			c.compBinaryExpr(n)
			fmt.Fprintln(c.fp, "movl(eax, edx);")
			fmt.Fprintln(c.fp, "popl(eax);")
		case *ast.CallExpr:
			fmt.Fprintln(c.fp, "pushl(eax);")
			c.compCallExpr(n)
			fmt.Fprintln(c.fp, "movl(eax, edx);")
			fmt.Fprintln(c.fp, "popl(eax);")
		case *ast.Ident:
			c.compIdent(n, "movl(ebp+%d, edx);\n")
		}
		switch b.Op {
		case token.ADD:
			fmt.Fprintln(c.fp, "addl(edx, eax);")
		case token.SUB:
			fmt.Fprintln(c.fp, "subl(edx, eax);")
		case token.MUL:
			fmt.Fprintln(c.fp, "mull(edx, eax);")
		case token.QUO:
			fmt.Fprintln(c.fp, "divl(edx, eax);")
		case token.REM:
			fmt.Fprintln(c.fp, "reml(edx, eax);")
		case token.AND:
			fmt.Fprintln(c.fp, "andl(eax, edx);")
		case token.EQL:
			fmt.Fprintln(c.fp, "eql(eax, edx);")
		case token.GTE:
			fmt.Fprintln(c.fp, "gel(eax, edx);")
		case token.GTT:
			fmt.Fprintln(c.fp, "gtl(eax, edx);")
		case token.LST:
			fmt.Fprintln(c.fp, "ltl(eax, edx);")
		case token.LTE:
			fmt.Fprintln(c.fp, "lel(eax, edx);")
		case token.NEQ:
			fmt.Fprintln(c.fp, "nel(eax, edx);")
		case token.OR:
			fmt.Fprintln(c.fp, "orl(eax, edx);")
		}
	}

	return 0
}

func (c *compiler) compCallExpr(e *ast.CallExpr) int {
	offset := 4

	ob := c.curScope.Lookup(e.Name.Name)
	switch {
	case e.Name.Name == "main":
		c.Error(e.Name.NamePos, "illegal to call function 'main'")
	case ob == nil:
		c.Error(e.Name.NamePos, "call to undeclared function '", e.Name.Name, "'")
	case ob.Kind != ast.Decl:
		c.Error(e.Name.NamePos, "may not call object that is not a function")
	case len(ob.Value.(*ast.DeclExpr).Params) != len(e.Args):
		c.Error(e.Name.NamePos, "number of arguments in function call do not "+
			"match declaration, expected ", len(ob.Value.(*ast.DeclExpr).Params),
			" got ", len(e.Args))
	}

	decl := ob.Value.(*ast.DeclExpr)
	for i, v := range e.Args {
		atype, dtype := typeOf(v, c.curScope), typeOf(decl.Params[i], decl.Scope)
		if atype != dtype {
			c.Error(e.Name.NamePos, "type mismatch, argument ", i, " of ",
				e.Name.Name, " is of type ", atype, " but expected ", dtype)
		}
	}
	for _, v := range e.Args {
		switch n := v.(type) {
		case *ast.BasicLit:
			c.compInt(n, fmt.Sprintf("esp+%d", offset))
		default:
			c.compNode(n)
			fmt.Fprintf(c.fp, "movl(eax, esp+%d);\n", offset)
		}
		offset += 4
	}
	fmt.Fprintf(c.fp, "_%s();\n", e.Name.Name)
	return 0
}

func (c *compiler) compDeclExpr(d *ast.DeclExpr) int {
	c.openScope(d.Scope)
	c.compScopeDecls()

	last := c.offset
	c.offset = 0
	for _, p := range d.Params {
		ob := c.curScope.Lookup(p.Name)
		ob.Offset = c.nextOffset()
	}
	x := c.countVars(d)
	fmt.Fprintf(c.fp, "void _%s(void) {\n", d.Name.Name)

	if x > 0 {
		fmt.Fprintf(c.fp, "enter(%d);\n", roundUp16(x))
		c.compNode(d.Body)
		fmt.Fprintln(c.fp, "leave();")
	} else {
		c.compNode(d.Body)
	}

	fmt.Fprintln(c.fp, "}")
	c.offset = last
	c.closeScope()
	return 0
}

func (c *compiler) compFile(f *ast.File) {
	c.topScope = f.Scope
	c.curScope = c.topScope
	c.compTopScope()
}

func (c *compiler) compIdent(n *ast.Ident, format string) {
	ob := c.curScope.Lookup(n.Name)
	if ob == nil {
		panic("no offset for identifier")
	}
	fmt.Fprintf(c.fp, format, ob.Offset)
}

func (c *compiler) compIfExpr(n *ast.IfExpr) {
	switch e := n.Cond.(type) {
	case *ast.BasicLit:
		c.compInt(e, "eax")
	case *ast.BinaryExpr:
		c.compBinaryExpr(e)
	}
	fmt.Fprintln(c.fp, "if (*(int32_t *)ecx == 1) {")
	c.openScope(n.Scope)
	c.compNode(n.Then)
	if n.Type != nil {
		fmt.Fprintln(c.fp, "leave();")
		fmt.Fprintln(c.fp, "return;")
	}
	if n.Else != nil && !reflect.ValueOf(n.Else).IsNil() {
		fmt.Fprintln(c.fp, "} else {")
		c.compNode(n.Else)
		if n.Type != nil {
			fmt.Fprintln(c.fp, "leave();")
			fmt.Fprintln(c.fp, "return;")
		}
	}
	c.closeScope()
	fmt.Fprintln(c.fp, "}")
}

func (c *compiler) compInt(n *ast.BasicLit, reg string) {
	i, err := strconv.Atoi(n.Lit)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Fprintf(c.fp, "setl(%d, %s);\n", i, reg)
}

func (c *compiler) compPackage(p *ast.Package) {
	c.topScope = p.Scope
	c.curScope = c.topScope
	c.compTopScope()
}

func (c *compiler) compScopeDecls() {
	for k, v := range c.curScope.Table {
		if v.Kind == ast.Decl {
			fmt.Fprintf(c.fp, "void _%s(void);\n", k)
			defer c.compNode(v.Value)
		}
	}
}

func (c *compiler) compTopScope() {
	ob := c.curScope.Lookup("main")
	switch {
	case ob == nil:
		c.Error(token.NoPos, "no entry point, function 'main' not found")
	case ob.Kind != ast.Decl:
		c.Error(ob.NamePos, "no entry point, 'main' is not a function")
	case ob.Type == nil:
		c.Error(ob.NamePos, "'main' must be of type int but was declared as "+
			"void")
	case ob.Type.Name != "int":
		c.Error(ob.Type.NamePos, "'main' must be of type but declared as ",
			ob.Type.Name)
	}
	fmt.Fprintln(c.fp, "#include <stdio.h>")
	fmt.Fprintln(c.fp, "#include <runtime.h>")
	c.compScopeDecls()
	fmt.Fprintln(c.fp, "int main(void) {")
	fmt.Fprintln(c.fp, "stack_init();")
	fmt.Fprintln(c.fp, "_main();")
	fmt.Fprintln(c.fp, "printf(\"%d\\n\", *(int32_t *)eax);")
	fmt.Fprintln(c.fp, "stack_end();")
	fmt.Fprintln(c.fp, "return *(int32_t*) eax;")
	fmt.Fprintln(c.fp, "}")
}

func (c *compiler) compVarExpr(v *ast.VarExpr) {
	ob := c.curScope.Lookup(v.Name.Name)
	ob.Offset = c.nextOffset()
	// TODO: value + infer type + check type
	if ob.Value != nil && !reflect.ValueOf(ob.Value).IsNil() {
		if val, ok := ob.Value.(*ast.AssignExpr); ok {
			c.compAssignExpr(val)
			return
		}
		panic("parsing error occured, object's Value is not an assignment")
	}
}

func (c *compiler) countVars(n ast.Node) (x int) {
	if n != nil && !reflect.ValueOf(n).IsNil() {
		switch e := n.(type) {
		case *ast.DeclExpr:
			x = len(e.Params)
			x += c.countVars(e.Body)
		case *ast.IfExpr:
			x = c.countVars(e.Then)
			x = c.countVars(e.Else)
		case *ast.ExprList:
			for _, v := range e.List {
				x += c.countVars(v)
			}
		case *ast.VarExpr:
			x = 1
		}
	}
	return
}
