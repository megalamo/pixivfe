// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/tools/go/packages"
)

// key models a gettext entry identified by context, singular msgid,
// and optional plural msgid_plural. For non-plural entries, plural is empty.
// NOTE: Comments, flags, or translator notes are currently not modeled.
type key struct {
	ctx    string
	id     string
	plural string
}

type ref struct {
	file string
	line int
}

// extractor holds the shared state and context for AST analysis within a package.
type extractor struct {
	refs        map[key][]ref
	projectRoot string
	fset        *token.FileSet
	info        *types.Info
	i18nPkgs    map[string]struct{}
}

func main() {
	outPath := flag.String("o", "po/pixivfe.pot", "output file")
	flag.Parse()

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get working directory: %v", err)
	}

	// We scan all buildable packages, including templ-generated Go sources.
	// templ-generated files must exist on disk before this runs.
	pkgs, err := packages.Load(&packages.Config{Mode: packages.LoadAllSyntax, Tests: false}, "./...")
	if err != nil {
		log.Fatalf("failed to load packages: %v", err)
	}

	if packages.PrintErrors(pkgs) > 0 {
		log.Fatal("failed to load packages due to errors")
	}

	refs := extractRefs(pkgs, findProjectRoot(wd), findI18nPkgPaths(pkgs))

	// Emit POT
	keys := make([]key, 0, len(refs))
	for k := range refs {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		if keys[i].ctx != keys[j].ctx {
			return keys[i].ctx < keys[j].ctx
		}

		if keys[i].id != keys[j].id {
			return keys[i].id < keys[j].id
		}

		return keys[i].plural < keys[j].plural
	})

	var b strings.Builder
	writeHeader(&b)

	for i, k := range keys {
		rs := refs[k]
		sort.Slice(rs, func(i, j int) bool {
			if rs[i].file != rs[j].file {
				return rs[i].file < rs[j].file
			}

			return rs[i].line < rs[j].line
		})

		// After sorting by file and line, duplicates will be adjacent.
		// Avoid a per-key set while producing identical output.
		fmt.Fprint(&b, "#:")

		lastFile := ""

		lastLine := 0
		for _, r := range rs {
			if r.file != lastFile || r.line != lastLine {
				fmt.Fprintf(&b, " %s:%d", r.file, r.line)

				lastFile = r.file
				lastLine = r.line
			}
		}

		fmt.Fprintln(&b)

		if k.ctx != "" {
			fmt.Fprintf(&b, "msgctxt %q\n", k.ctx)
		}

		// Plural or singular entry
		if k.plural != "" {
			fmt.Fprintf(&b, "msgid %q\n", k.id)
			fmt.Fprintf(&b, "msgid_plural %q\n", k.plural)
			fmt.Fprintf(&b, "msgstr[0] \"\"\n")
			fmt.Fprintf(&b, "msgstr[1] \"\"\n")
		} else {
			fmt.Fprintf(&b, "msgid %q\n", k.id)
			fmt.Fprintf(&b, "msgstr \"\"\n")
		}

		// Add a separating blank line, but not after the very last entry.
		if i < len(keys)-1 {
			fmt.Fprintln(&b)
		}
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		log.Fatalf("failed to create output directory: %v", err)
	}

	if err := os.WriteFile(*outPath, []byte(b.String()), 0o644); err != nil {
		log.Fatalf("failed to write output file %s: %v", *outPath, err)
	}
}

// extractRefs traverses all Go source files in the given packages,
// looking for i18n function calls and message keys to extract.
func extractRefs(pkgs []*packages.Package, projectRoot string, i18nPkgPaths map[string]struct{}) map[key][]ref {
	refs := map[key][]ref{}

	for _, p := range pkgs {
		if p.TypesInfo == nil {
			continue
		}

		// Create an extractor with the context for this package's files.
		e := &extractor{
			refs:        refs,
			projectRoot: projectRoot,
			fset:        p.Fset,
			info:        p.TypesInfo,
			i18nPkgs:    i18nPkgPaths,
		}

		for _, f := range p.Syntax {
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.CallExpr:
					e.handleCallExpr(x)
				case *ast.CompositeLit:
					e.handleCompositeLit(x)
				}

				return true
			})
		}
	}

	return refs
}

// findI18nPkgPaths returns the set of package paths in this build that
// define the i18n package with a MsgKey type whose underlying type is string.
// This lets us require that matched Tr/TrN/TrC/TrNC calls, and MsgKey conversions,
// come from our i18n package, regardless of how it is imported or aliased.
func findI18nPkgPaths(pkgs []*packages.Package) map[string]struct{} {
	out := make(map[string]struct{})

	for _, p := range pkgs {
		// We are looking for the local i18n package.
		// The package name is "i18n", and it must define a MsgKey whose
		// underlying type is string.
		if p.Name != "i18n" || p.Types == nil {
			continue
		}

		obj := p.Types.Scope().Lookup("MsgKey")

		tn, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}

		named, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}

		basic, ok := named.Underlying().(*types.Basic)
		if ok && basic.Kind() == types.String {
			out[p.PkgPath] = struct{}{}
		}
	}

	return out
}

// constString evaluates expr to a constant string if possible using types.Info.
// Handles string literals, const identifiers, and constant expressions like "a" + "b".
// Non-constant expressions return false.
func constString(info *types.Info, expr ast.Expr) (string, bool) {
	tv, ok := info.Types[expr]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}

	return constant.StringVal(tv.Value), true
}

// isMsgKeyNamedTypeInI18n reports whether t is exactly the named type i18n.MsgKey,
// with package path present in i18nPkgs.
// Accepts both direct types and type aliases that resolve to i18n.MsgKey.
func isMsgKeyNamedTypeInI18n(t types.Type, i18nPkgs map[string]struct{}) bool {
	// For a type alias, the TypeName.Type() is the aliased type, so this check
	// still sees the real named type object behind the alias.
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}

	if _, ok := i18nPkgs[obj.Pkg().Path()]; !ok {
		return false
	}

	return obj.Name() == "MsgKey"
}

// handleCompositeLit inspects composite literals to find implicit conversions to i18n.MsgKey.
func (e *extractor) handleCompositeLit(x *ast.CompositeLit) {
	tv, ok := e.info.Types[x]
	if !ok || tv.Type == nil {
		return
	}

	// Unwrap one level of pointer so &T{...} is treated as T{...}.
	t := tv.Type
	if p, ok := t.Underlying().(*types.Pointer); ok && p.Elem() != nil {
		t = p.Elem()
	}

	switch u := t.Underlying().(type) {
	case *types.Map:
		keyIsMK := isMsgKeyNamedTypeInI18n(u.Key(), e.i18nPkgs)

		valIsMK := isMsgKeyNamedTypeInI18n(u.Elem(), e.i18nPkgs)
		if !keyIsMK && !valIsMK {
			return
		}

		for _, elt := range x.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}

			if keyIsMK {
				if msg, ok := constString(e.info, kv.Key); ok {
					e.addRef(kv.Key.Pos(), msg, "", "")
				}
			}

			if valIsMK {
				if msg, ok := constString(e.info, kv.Value); ok {
					e.addRef(kv.Value.Pos(), msg, "", "")
				}
			}
		}

	case *types.Slice, *types.Array:
		var elemType types.Type
		if s, ok := u.(*types.Slice); ok {
			elemType = s.Elem()
		} else {
			// If not a slice, it must be an array due to the case statement.
			elemType = u.(*types.Array).Elem()
		}

		if !isMsgKeyNamedTypeInI18n(elemType, e.i18nPkgs) {
			return
		}

		for _, elt := range x.Elts {
			if msg, ok := constString(e.info, elt); ok {
				e.addRef(elt.Pos(), msg, "", "")
			}
		}

	case *types.Struct:
		// To handle both keyed and positional literals, we first map field names to their types.
		// Then, for keyed elements we look up the type by name. For positional elements, we
		// rely on the declared field order.
		fieldTypes := make(map[string]types.Type, u.NumFields())
		for i := range u.NumFields() {
			f := u.Field(i)

			fieldTypes[f.Name()] = f.Type()
		}

		for i, elt := range x.Elts {
			// Keyed field: FieldName: "..."
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				if id, ok := kv.Key.(*ast.Ident); ok {
					if ft, ok := fieldTypes[id.Name]; ok && isMsgKeyNamedTypeInI18n(ft, e.i18nPkgs) {
						if msg, ok := constString(e.info, kv.Value); ok {
							e.addRef(kv.Value.Pos(), msg, "", "")
						}
					}
				}

				continue
			}

			// Positional field: rely on declared field order.
			if i < u.NumFields() {
				ft := u.Field(i).Type()
				if isMsgKeyNamedTypeInI18n(ft, e.i18nPkgs) {
					if msg, ok := constString(e.info, elt); ok {
						e.addRef(elt.Pos(), msg, "", "")
					}
				}
			}
		}
	}
}

// handleCallExpr inspects function calls and type conversions to find i18n messages.
func (e *extractor) handleCallExpr(x *ast.CallExpr) {
	// Case 1: Type conversion, e.g., i18n.MsgKey("Hello").
	// A call expression where x.Fun is a type is a type conversion.
	if tv, ok := e.info.Types[x.Fun]; ok && tv.IsType() {
		if len(x.Args) == 1 && isMsgKeyNamedTypeInI18n(tv.Type, e.i18nPkgs) {
			if msg, ok := constString(e.info, x.Args[0]); ok {
				e.addRef(x.Args[0].Pos(), msg, "", "")
			}
		}

		return // This was a type conversion, handled or not.
	}

	// Case 2: For function calls, first check if it's one of the special Tr* family.
	// These have specific argument structures for msgid, context, and plurals.
	if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
		if fn, ok := e.info.Uses[sel.Sel].(*types.Func); ok && fn.Pkg() != nil {
			if _, ok := e.i18nPkgs[fn.Pkg().Path()]; ok {
				switch fn.Name() {
				case "NewUserError": // NewUserError(ctx, "msg", ...)
					if len(x.Args) >= 2 {
						if msg, ok := constString(e.info, x.Args[1]); ok {
							e.addRef(x.Args[1].Pos(), msg, "", "")
						}
					}

					return // Handled as NewUserError call.
				case "Tr": // Tr(ctx, "msg", ...)
					if len(x.Args) >= 2 {
						if msg, ok := constString(e.info, x.Args[1]); ok {
							e.addRef(x.Args[1].Pos(), msg, "", "")
						}
					}

					return // Handled as Tr call.
				case "TrC": // TrC(ctx, "ctx", "msg", ...)
					if len(x.Args) >= 3 {
						ctx, ok1 := constString(e.info, x.Args[1])

						msg, ok2 := constString(e.info, x.Args[2])
						if ok1 && ok2 {
							e.addRef(x.Args[2].Pos(), msg, ctx, "")
						}
					}

					return // Handled as TrC call.
				case "TrN": // TrN(ctx, "singular", "plural", n, ...)
					if len(x.Args) >= 4 {
						singular, ok1 := constString(e.info, x.Args[1])

						plural, ok2 := constString(e.info, x.Args[2])
						if ok1 && ok2 {
							e.addRef(x.Args[1].Pos(), singular, "", plural)
						}
					}

					return // Handled as TrN call.
				case "TrNC": // TrNC(ctx, "ctx", "singular", "plural", n, ...)
					if len(x.Args) >= 5 {
						ctx, ok1 := constString(e.info, x.Args[1])
						singular, ok2 := constString(e.info, x.Args[2])

						plural, ok3 := constString(e.info, x.Args[3])
						if ok1 && ok2 && ok3 {
							e.addRef(x.Args[2].Pos(), singular, ctx, plural)
						}
					}

					return // Handled as TrNC call.
				}
			}
		}
	}

	// Case 3: A generic function call with i18n.MsgKey parameters.
	// This handles implicit conversions for any function taking an i18n.MsgKey.
	// We use TypeOf because it works for qualified (pkg.Func) and unqualified (Func) calls.
	sig, ok := e.info.TypeOf(x.Fun).(*types.Signature)
	if !ok {
		return
	}

	params := sig.Params()

	n := params.Len()
	if n == 0 {
		return
	}

	variadic := sig.Variadic()
	last := n - 1

	for i, arg := range x.Args {
		var pt types.Type

		if variadic && i >= last {
			// If called with ...slice, let composite literal handling discover elements.
			if x.Ellipsis != token.NoPos {
				continue
			}
			// A valid variadic signature guarantees the last param is a slice.
			pt = params.At(last).Type().(*types.Slice).Elem()
		} else {
			if i >= n {
				break // More arguments than parameters (and not variadic)
			}

			pt = params.At(i).Type()
		}

		if isMsgKeyNamedTypeInI18n(pt, e.i18nPkgs) {
			if msg, ok := constString(e.info, arg); ok {
				e.addRef(arg.Pos(), msg, "", "")
			}
		}
	}
}

// addRef records a reference to a msgid, normalising the file path relative
// to the computed project root.
func (e *extractor) addRef(pos token.Pos, msg, ctx, plural string) {
	p := e.fset.Position(pos)

	file := p.Filename
	if rel, err := filepath.Rel(e.projectRoot, file); err == nil {
		file = rel
	}

	file = filepath.ToSlash(file)

	k := key{ctx: ctx, id: msg, plural: plural}

	e.refs[k] = append(e.refs[k], ref{file: file, line: p.Line})
}

// writeHeader emits a POT header.
func writeHeader(b *strings.Builder) {
	fmt.Fprintln(b, `msgid ""`)
	fmt.Fprintln(b, `msgstr ""`)
	fmt.Fprintf(b, "\"Project-Id-Version: PixivFE %s\\n\"\n", detectVersion())
	fmt.Fprintf(b, "\"POT-Creation-Date: %s\\n\"\n", time.Now().UTC().Format("2006-01-02 15:04+0000"))
	fmt.Fprintln(b, `"Language: en\n"`)
	fmt.Fprintln(b, `"SPDX-License-Identifier: GFDL-1.3-only\n"`)
	fmt.Fprintln(b, `"Report-Msgid-Bugs-To: https://codeberg.org/pixivfe/pixivfe/issues\n"`)
	fmt.Fprintln(b, `"MIME-Version: 1.0\n"`)
	fmt.Fprintln(b, `"Content-Type: text/plain; charset=UTF-8\n"`)
	fmt.Fprintln(b, `"Content-Transfer-Encoding: 8bit\n"`)
	fmt.Fprintln(b, `"Plural-Forms: nplurals=2; plural=(n != 1);\n"`)
	fmt.Fprintln(b)
}

// detectVersion resolves a human-friendly version string using git describe.
// Falls back to "dev" when git is unavailable or this is not a git checkout.
func detectVersion() string {
	cmd := exec.Command("git", "describe", "--tags", "--always", "--dirty")

	out, err := cmd.Output()
	if err != nil {
		return "dev"
	}

	return strings.TrimSpace(string(out))
}

// findProjectRoot attempts to find a stable root directory for source references.
// Preference order:
//  1. git toplevel directory
//  2. nearest parent directory that contains go.mod
//  3. the provided working directory
func findProjectRoot(wd string) string {
	// Try git toplevel
	if root := gitTopLevel(wd); root != "" {
		return root
	}
	// Fall back to nearest go.mod
	if root := nearestGoModDir(wd); root != "" {
		return root
	}

	return wd
}

func gitTopLevel(wd string) string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")

	cmd.Dir = wd

	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	root := strings.TrimSpace(string(out))
	if root == "" {
		return ""
	}

	return filepath.Clean(root)
}

func nearestGoModDir(start string) string {
	dir := filepath.Clean(start)
	for {
		if fileExists(filepath.Join(dir, "go.mod")) {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return ""
}

func fileExists(path string) bool {
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		return true
	}

	return false
}
