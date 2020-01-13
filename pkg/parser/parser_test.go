package parser_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/SafetyCulture/djinni-parser/pkg/ast"
	"github.com/SafetyCulture/djinni-parser/pkg/parser"
)

func TestImports(t *testing.T) {
	t.Parallel()
	src := `
		@import "relative/path/to/filename.djinni"
		@import "relative/path/to/filename2.djinni"
	`

	f, err := parser.ParseFile("", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(f.Imports) != 2 {
		t.Fatalf("incorrect number of imports; expected 2, but got %d", len(f.Imports))
	}

	if f.Imports[0] != "relative/path/to/filename.djinni" {
		t.Errorf("incorrect import path: %q", f.Imports[0])
	}
	if f.Imports[1] != "relative/path/to/filename2.djinni" {
		t.Errorf("incorrect import path: %q", f.Imports[1])
	}
}

func TestTypeDecls(t *testing.T) {
	t.Parallel()

	tests := [...]struct {
		name  string
		src   string
		ident string
		want  interface{}
	}{
		{"EmptyRecord", "myRecord = record {}", "myRecord", &ast.Record{}},
		{"EmptyRecordWithExt", "myRecord = record +o +j {}", "myRecord", &ast.Record{Ext: ast.Ext{ObjC: true, Java: true}}},
		{"EmptyEnum", "my_enum = enum {}", "my_enum", &ast.Enum{}},
		{"EmptyFlags", "my_flags = flags {}", "my_flags", &ast.Enum{Flags: true}},
		{"EmptyCPPInterface", "my_cpp_interface = interface +c {}", "my_cpp_interface", &ast.Interface{Ext: ast.Ext{CPP: true}}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f, err := parser.ParseFile("", tt.src)
			if err != nil {
				t.Fatal(err)
			}
			if len(f.TypeDecls) != 1 {
				t.Fatalf("incorrect number of decls; expected 1, got %d:\n%#v", len(f.TypeDecls), f.TypeDecls)
			}

			d := f.TypeDecls[0]
			if d.Ident.Name != tt.ident {
				t.Errorf("incorrect identifier: expected %q, got %q", tt.ident, d.Ident.Name)
			}

			diff := cmp.Diff(tt.want, d.Body)
			if diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}

func TestRecordFields(t *testing.T) {
	t.Parallel()

	tests := [...]struct {
		name string
		src  string
		want *ast.Record
	}{
		{"i32", "my_record = record { id: i32; }",
			&ast.Record{
				Fields: []ast.Field{
					ast.Field{
						Ident: ast.Ident{Name: "id"},
						Type:  ast.TypeExpr{Ident: ast.Ident{Name: "i32"}},
					},
				},
				Consts: nil,
			},
		},
		{"optional", "my_record = record { opt: optional<string>; }",
			&ast.Record{
				Fields: []ast.Field{
					ast.Field{
						Ident: ast.Ident{Name: "opt"},
						Type: ast.TypeExpr{
							Ident: ast.Ident{Name: "optional"},
							Args: []ast.TypeExpr{
								ast.TypeExpr{
									Ident: ast.Ident{Name: "string"},
								},
							},
						},
					},
				},
				Consts: nil,
			},
		},
		{"map", "my_record = record { ma: map<string, i32>; }",
			&ast.Record{
				Fields: []ast.Field{
					ast.Field{
						Ident: ast.Ident{Name: "ma"},
						Type: ast.TypeExpr{
							Ident: ast.Ident{Name: "map"},
							Args: []ast.TypeExpr{
								ast.TypeExpr{
									Ident: ast.Ident{Name: "string"},
								},
								ast.TypeExpr{
									Ident: ast.Ident{Name: "i32"},
								},
							},
						},
					},
				},
				Consts: nil,
			},
		},
		{"const", "my_record = record { const string_const: string = \"Constants can be put here\"; }",
			&ast.Record{
				Fields: nil,
				Consts: []ast.Const{
					ast.Const{
						Doc: nil,
						Ident: ast.Ident{
							Name: "string_const",
						},
						Type: ast.TypeExpr{
							Ident: ast.Ident{
								Name: "string",
							},
						},
						Value: interface{}("Constants can be put here"),
					},
				},
			},
		},
		{"const_emptyf", "my_record = record { const string_const: string = \"\"; }",
			&ast.Record{
				Fields: nil,
				Consts: []ast.Const{
					ast.Const{
						Doc: nil,
						Ident: ast.Ident{
							Name: "string_const",
						},
						Type: ast.TypeExpr{
							Ident: ast.Ident{
								Name: "string",
							},
						},
						Value: interface{}(""),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f, err := parser.ParseFile("", tt.src)
			if err != nil {
				t.Fatal(err)
			}
			if len(f.TypeDecls) != 1 {
				t.Fatalf("incorrect number of decls; expected 1, got %d:\n%#v", len(f.TypeDecls), f.TypeDecls)
			}

			d := f.TypeDecls[0]
			diff := cmp.Diff(tt.want, d.Body)
			if diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}

func TestEnum(t *testing.T) {
	t.Parallel()

	tests := [...]struct {
		name string
		src  string
		want *ast.Enum
	}{
		{"enum", "my_record = enum { option1; }",
			&ast.Enum{
				Options: []ast.EnumOption{
					ast.EnumOption{
						Ident: ast.Ident{Name: "option1"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f, err := parser.ParseFile("", tt.src)
			if err != nil {
				t.Fatal(err)
			}
			if len(f.TypeDecls) != 1 {
				t.Fatalf("incorrect number of decls; expected 1, got %d:\n%#v", len(f.TypeDecls), f.TypeDecls)
			}

			d := f.TypeDecls[0]
			diff := cmp.Diff(tt.want, d.Body)
			if diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}
