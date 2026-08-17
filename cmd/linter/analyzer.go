package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

// analyzer сообщает о вызовах panic, а также log.Fatal*/os.Exit вне main.main.
var analyzer = &analysis.Analyzer{
	Name:     "exitcheck",
	Doc:      "reports panic and log.Fatal/os.Exit calls outside main.main",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Body == nil {
			return
		}

		inMainMain := pass.Pkg.Name() == "main" &&
			fn.Recv == nil &&
			fn.Name != nil &&
			fn.Name.Name == "main"

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			reportForbiddenCall(pass, call, inMainMain)
			return true
		})
	})

	return nil, nil
}

func reportForbiddenCall(pass *analysis.Pass, call *ast.CallExpr, inMainMain bool) {
	if isBuiltinPanic(pass, call) {
		pass.Reportf(call.Pos(), "panic is forbidden")
		return
	}

	if inMainMain {
		return
	}

	switch name := calleeFullName(pass, call); name {
	case "log.Fatal", "log.Fatalf", "log.Fatalln":
		pass.Reportf(call.Pos(), "%s is forbidden outside main function of package main", name)
	case "os.Exit":
		pass.Reportf(call.Pos(), "os.Exit is forbidden outside main function of package main")
	}
}

func isBuiltinPanic(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != "panic" {
		return false
	}
	obj, ok := pass.TypesInfo.Uses[ident]
	if !ok {
		return false
	}
	_, ok = obj.(*types.Builtin)
	return ok
}

func calleeFullName(pass *analysis.Pass, call *ast.CallExpr) string {
	obj := typeutil.Callee(pass.TypesInfo, call)
	fn, ok := obj.(*types.Func)
	if !ok {
		return ""
	}
	return fn.FullName()
}
