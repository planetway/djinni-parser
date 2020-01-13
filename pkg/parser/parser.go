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
		p.errorf("expected %q, got %q", tok, p.tok)
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

func (p *parser) parseRecord() *ast.Record {
	p.next()
	ext := p.parseLangExt()
	p.expect(token.LBRACE)

	fields := []ast.Field{}
	consts := []ast.Const{}
	for p.tok != token.RBRACE && p.tok != token.EOF {
		log.Printf("p.tok=%v,p.lit=%v", p.tok, p.lit)
		if p.tok == token.CONST {
			// TODO
		} else if p.tok == token.IDENT {
			// is a record field
			field := p.parseRecordField()
			if field != nil {
				fields = append(fields, *field)
			}
		} else {
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

// Parse record fields,
// ex: "id: i32;"
func (p *parser) parseRecordField() *ast.Field {
	ident := ast.Ident{
		Name: p.lit,
	}
	p.next()

	p.expect(token.COLON)

	var typeExpr ast.TypeExpr
	if p.tok == token.MAP {
		if t := p.parseMap(); t != nil {
			typeExpr = *t
		}
	} else if p.tok == token.SET {
		// TODO
	} else if p.tok == token.LIST {
		// TODO
	} else if p.tok == token.IDENT {
		// TODO later we want to check if all types exist, including the ones refer to custom records
		typeExpr = ast.TypeExpr{
			Ident: ast.Ident{
				Name: p.lit,
			},
		}
	} else {
		p.errorf("unexpected token: %q", p.tok)
	}
	p.next()

	p.expect(token.SEMICOLON)

	return &ast.Field{
		Doc:   nil, // TODO
		Ident: ident,
		Type:  typeExpr,
	}
}

func (p *parser) parseMap() *ast.TypeExpr {
	p.expect(token.LANGLE)
	if p.tok != token.IDENT {
		p.errorf("expected IDENT, got %q", p.tok)
		return nil
	}
	l := ast.Ident{
		Name: p.lit,
	}
	p.expect(token.COMMA)
	if p.tok != token.IDENT {
		p.errorf("expected IDENT, got %q", p.tok)
		return nil
	}
	r := ast.Ident{
		Name: p.lit,
	}
	p.expect(token.RANGLE)
	p.expect(token.SEMICOLON)
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

func (p *parser) parseEnum(isFlags bool) *ast.Enum {
	p.expect(token.LBRACE)

	// TODO: handle all options
	for p.tok != token.RBRACE && p.tok != token.EOF {
		p.next()
	}

	p.expect(token.RBRACE)

	return &ast.Enum{
		Flags: isFlags,
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
		return p.parseRecord()
	case token.INTERFACE:
		return p.parseInterface()
	case token.ENUM:
		return p.parseEnum(false)
	case token.FLAGS:
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
