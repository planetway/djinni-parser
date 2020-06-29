// Package parser implements a parser for Djinni IDL source files.
//
package parser

import (
	"fmt"
	"log"

	"github.com/SafetyCulture/djinni-parser/pkg/ast"
	"github.com/SafetyCulture/djinni-parser/pkg/scanner"
	"github.com/SafetyCulture/djinni-parser/pkg/token"
)

type parser struct {
	scanner scanner.Scanner

	tok token.Token // last read token
	lit string      // token literal

	leadComment *ast.CommentGroup // last lead comment

	errors errorsList
}

func (p *parser) init(src []byte) {
	p.scanner.Init(src)
	p.next()
}

func (p *parser) next() {
	p.leadComment = nil

	p.tok, p.lit = p.scanner.Scan()

	if p.tok == token.COMMENT {
		// TODO: Consume the comments to parse as docs for the next token
		p.next()
	}
}

func (p *parser) errorf(msg string, args ...interface{}) {

	// Track all errors and continue parsing.
	p.errors.add(fmt.Sprintf(msg, args...))
	log.Printf(msg, args...)

	// bailout if too many errors
	if len(p.errors) > 10 {
		// TODO: bailout
	}
}

func (p *parser) expect(tok token.Token) {
	if p.tok != tok {
		p.errorf("expected %q, got %q, p: %#v", tok, p.tok, p)
	}
	p.next()
}

func (p *parser) parseImport() (i string) {
	p.next()
	if p.tok != token.STRING {
		p.expect(token.STRING)
		return
	}
	// strip the quotes
	i = string(p.lit[1 : len(p.lit)-1])
	p.next()
	return
}

func (p *parser) parseLangExt() ast.Ext {
	ext := ast.Ext{}
	if !p.tok.IsLangExt() {
		return ext
	}
	for p.tok.IsLangExt() {
		switch p.tok {
		case token.CPP:
			ext.CPP = true
		case token.OBJC:
			ext.ObjC = true
		case token.JAVA:
			ext.Java = true
		}
		p.next()
	}
	return ext
}

// ex: "+c +j +o { ... }"
func (p *parser) parseRecord() *ast.Record {
	ext := p.parseLangExt()
	p.expect(token.LBRACE)

	var fields []ast.Field
	var consts []ast.Const

	for p.tok != token.RBRACE && p.tok != token.EOF {
		if p.tok == token.CONST {
			cnst := p.parseRecordConst()
			if cnst != nil {
				consts = append(consts, *cnst)
			}
		} else if p.tok == token.IDENT {
			field := p.parseRecordField()
			if field != nil {
				fields = append(fields, *field)
			}
		} else {
			p.errorf("unhandled token: %q", p.tok)
			p.next()
		}
	}

	p.expect(token.RBRACE)

	return &ast.Record{
		Ext:    ext,
		Fields: fields,
		Consts: consts,
	}
}

// Parse record fields.
// ex: "id: i32;"
// ex: "id: optional<list<string>>;"
func (p *parser) parseRecordField() *ast.Field {
	ident := ast.Ident{
		Name: p.lit,
	}
	p.next()

	p.expect(token.COLON)

	typeExpr := p.parseRecordType()
	if typeExpr == nil {
		p.errorf("unexpected token: %q", p.tok)
		p.next()
		return nil
	}

	p.expect(token.SEMICOLON)

	return &ast.Field{
		Doc:   nil, // TODO
		Ident: ident,
		Type:  *typeExpr,
	}
}

// ex: "i32"
// ex: "optional<list<string>>"
func (p *parser) parseRecordType() *ast.TypeExpr {
	var typeExpr *ast.TypeExpr

	if p.tok == token.MAP {
		p.next()
		typeExpr = p.parseMap()
	} else if p.tok == token.SET {
		p.next()
		typeExpr = p.parseDecorated("set")
	} else if p.tok == token.LIST {
		p.next()
		typeExpr = p.parseDecorated("list")
	} else if p.tok == token.IDENT && p.lit == "optional" {
		p.next()
		typeExpr = p.parseDecorated("optional")
	} else if p.tok == token.IDENT {
		// TODO later we want to check if all types exist, including the ones refer to custom records
		typeExpr = &ast.TypeExpr{
			Ident: ast.Ident{
				Name: p.lit,
			},
		}
		p.next()
	}
	return typeExpr
}

// Parse record constants.
// ex: "const string_const: string = \"Constants can be put here\";"
func (p *parser) parseRecordConst() *ast.Const {
	// skip the "const"
	p.next()

	if p.tok != token.IDENT {
		p.errorf("expected IDENT but got: %q", p.tok)
		return nil
	}
	ident := ast.Ident{
		Name: p.lit,
	}
	p.next()

	p.expect(token.COLON)

	if p.tok != token.IDENT {
		p.errorf("expected IDENT but got: %q", p.tok)
		return nil
	}

	// TODO later we want to check if all types exist, including the ones refer to custom records
	typeExpr := ast.TypeExpr{
		Ident: ast.Ident{
			Name: p.lit,
		},
	}
	p.next()

	p.expect(token.ASSIGN)

	if p.tok == token.LBRACE {
		// TODO support constant custom record
		p.errorf("skipping constant custom record")
		for p.tok != token.RBRACE && p.tok != token.EOF {
			p.next()
		}
	} else if p.tok != token.INT && p.tok != token.FLOAT && p.tok != token.STRING {
		p.errorf("unexpected token: %q", p.tok)
		return nil
	}

	var val interface{}
	if p.tok == token.STRING {
		// remove the first and last "
		val = p.lit[1 : len(p.lit)-1]
	} else {
		val = p.lit
	}
	p.next()

	p.expect(token.SEMICOLON)

	return &ast.Const{
		Doc:   nil, // TODO
		Ident: ident,
		Type:  typeExpr,
		Value: val,
	}
}

// Parse the content inside the generic set/list/optional types
// ex: <IDENT>
func (p *parser) parseDecorated(name string) *ast.TypeExpr {
	p.expect(token.LANGLE)

	typeExpr := p.parseRecordType()
	if typeExpr == nil {
		p.errorf("unexpected token: %q", p.tok)
		return nil
	}

	p.expect(token.RANGLE)

	return &ast.TypeExpr{
		Ident: ast.Ident{
			Name: name,
		},
		Args: []ast.TypeExpr{*typeExpr},
	}
}

// Parse map types.
// ex: "<string, i32>"
func (p *parser) parseMap() *ast.TypeExpr {
	p.expect(token.LANGLE)

	if p.tok != token.IDENT {
		p.errorf("expected IDENT, got %q", p.tok)
		return nil
	}
	l := ast.Ident{
		Name: p.lit,
	}
	p.next()

	p.expect(token.COMMA)

	if p.tok != token.IDENT {
		p.errorf("expected IDENT, got %q", p.tok)
		return nil
	}
	r := ast.Ident{
		Name: p.lit,
	}
	p.next()

	p.expect(token.RANGLE)

	return &ast.TypeExpr{
		Ident: ast.Ident{
			Name: "map",
		},
		Args: []ast.TypeExpr{
			ast.TypeExpr{
				Ident: l,
			},
			ast.TypeExpr{
				Ident: r,
			},
		},
	}
}

func (p *parser) parseInterface() *ast.Interface {
	p.next()
	ext := p.parseLangExt()
	p.expect(token.LBRACE)

	// TODO: handle all the interface methods
	for p.tok != token.RBRACE && p.tok != token.EOF {
		p.next()
	}

	p.expect(token.RBRACE)

	return &ast.Interface{
		Ext: ext,
	}
}

// ex: "{ ... }"
func (p *parser) parseEnum(isFlags bool) *ast.Enum {
	p.expect(token.LBRACE)

	var options []ast.EnumOption

	for p.tok != token.RBRACE && p.tok != token.EOF {
		if p.tok == token.IDENT {
			option := ast.EnumOption{
				Doc: nil, // TODO
				Ident: ast.Ident{
					Name: p.lit,
				},
			}
			options = append(options, option)
			p.next()
			p.expect(token.SEMICOLON)
		} else {
			p.errorf("unhandled token: %q", p.tok)
			p.next()
		}
	}

	p.expect(token.RBRACE)

	return &ast.Enum{
		Options: options,
		Flags:   isFlags,
	}
}

func (p *parser) parseIdent() ast.Ident {
	name := "_"
	if p.tok == token.IDENT {
		name = p.lit
		p.next()
	} else {
		p.expect(token.IDENT)
	}

	return ast.Ident{Name: name}
}

func (p *parser) parseTypeDef() ast.TypeDef {
	if !p.tok.IsTypeDef() {
		p.errorf("expected one of %v, got %q", token.TypeDefTokens(), p.tok)
		p.next()
	}

	switch p.tok {
	case token.RECORD:
		p.next()
		return p.parseRecord()
	case token.INTERFACE:
		return p.parseInterface()
	case token.ENUM:
		p.next()
		return p.parseEnum(false)
	case token.FLAGS:
		p.next()
		return p.parseEnum(true)
	default:
		return nil
	}
}

// All decls should be in the form IDENT = KEYWORD [EXT] { }
func (p *parser) parseDecl() (decl ast.TypeDecl) {
	decl.Ident = p.parseIdent()
	p.expect(token.ASSIGN)
	decl.Body = p.parseTypeDef()
	return
}

func (p *parser) parseFile() *ast.IDLFile {

	// import decls
	var imports []string
	for p.tok == token.IMPORT {
		imports = append(imports, p.parseImport())
	}

	// rest of body
	var decls []ast.TypeDecl
	for p.tok != token.EOF {
		decls = append(decls, p.parseDecl())
	}

	return &ast.IDLFile{
		Imports:   imports,
		TypeDecls: decls,
	}
}
