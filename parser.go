package esi

import (
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/nussjustin/esi/esixml"
)

// DuplicateElementError is returned when multiple elements with the same name are found where only one is allowed.
type DuplicateElementError struct {
	Position Position

	// Name is the element name.
	Name esixml.Name
}

// Error returns a human-readable error message.
func (d *DuplicateElementError) Error() string {
	return fmt.Sprintf(`duplicate element %s at position %s`, d.Name, d.Position)
}

// Is checks if the given error matches the receiver.
func (d *DuplicateElementError) Is(err error) bool {
	var o *DuplicateElementError
	return errors.As(err, &o) && *o == *d
}

// EmptyElementError is returned when an element that requires content is specified as self-closed.
type EmptyElementError struct {
	Position Position

	// Element is the element name.
	Name esixml.Name
}

// Error returns a human-readable error message.
func (e *EmptyElementError) Error() string {
	return fmt.Sprintf(
		`element %s is empty at position %s`,
		e.Name,
		e.Position,
	)
}

// Is checks if the given error matches the receiver.
func (e *EmptyElementError) Is(err error) bool {
	var o *EmptyElementError
	return errors.As(err, &o) && *o == *e
}

// InvalidAttributeValueError is returned when an attribute value is invalid for the specific attribute.
type InvalidAttributeValueError struct {
	Position Position

	// Element is the element name.
	Element esixml.Name

	// Name is the attribute name.
	Name esixml.Name

	// Value is the provided invalid value.
	Value string

	// Allowed contains a list of allowed values for the attribute, if known.
	Allowed []string
}

// Error returns a human-readable error message.
func (i *InvalidAttributeValueError) Error() string {
	return fmt.Sprintf(
		`invalid value %q for attribute %s in element %s at position %s`,
		i.Value,
		i.Name,
		i.Element,
		i.Position,
	)
}

// Is checks if the given error matches the receiver.
func (i *InvalidAttributeValueError) Is(err error) bool {
	var o *InvalidAttributeValueError
	return errors.As(err, &o) && o.Element == i.Element && o.Name == i.Name && o.Value == i.Value && slices.Equal(o.Allowed, i.Allowed) //nolint:lll
}

// InvalidElementError is returned when an invalid <esi:*> element is found.
type InvalidElementError struct {
	Position Position

	// Name is the element name.
	Name esixml.Name
}

// Error returns a human-readable error message.
func (i *InvalidElementError) Error() string {
	return fmt.Sprintf(`invalid element %s at position %s`, i.Name, i.Position)
}

// Is checks if the given error matches the receiver.
func (i *InvalidElementError) Is(err error) bool {
	var o *InvalidElementError
	return errors.As(err, &o) && *o == *i
}

// MissingAttributeError is returned when a required attribute is missing on an element.
type MissingAttributeError struct {
	Position Position

	// Element is the element name.
	Element esixml.Name

	// Attribute is the name of the attribute.
	Attribute esixml.Name
}

// Error returns a human-readable error message.
func (m *MissingAttributeError) Error() string {
	return fmt.Sprintf(`missing attribute %s in element %s at position %s`, m.Attribute, m.Element, m.Position)
}

// Is checks if the given error matches the receiver.
func (m *MissingAttributeError) Is(err error) bool {
	var o *MissingAttributeError
	return errors.As(err, &o) && *o == *m
}

// MissingElementError is returned when a required child element is not found inside another element.
type MissingElementError struct {
	Position Position

	// Name is the element name.
	Name esixml.Name
}

// Error returns a human-readable error message.
func (m *MissingElementError) Error() string {
	return fmt.Sprintf(`missing element %s at position %s`, m.Name, m.Position)
}

// Is checks if the given error matches the receiver.
func (m *MissingElementError) Is(err error) bool {
	var o *MissingElementError
	return errors.As(err, &o) && *o == *m
}

// UnclosedElementError is returned when an empty element is not closed directly.
type UnclosedElementError struct {
	Position Position

	// Name is the element name.
	Name esixml.Name
}

// Error returns a human-readable error message.
func (u *UnclosedElementError) Error() string {
	return fmt.Sprintf(`unclosed element %s at position %s`, u.Name, u.Position)
}

// Is checks if the given error matches the receiver.
func (u *UnclosedElementError) Is(err error) bool {
	var o *UnclosedElementError
	return errors.As(err, &o) && *o == *u
}

// UnexpectedElementError is returned when a specific element was expected, but a different one was encountered.
type UnexpectedElementError struct {
	Position Position

	// Name is the encountered element name.
	Name esixml.Name
}

// Error returns a human-readable error message.
func (u *UnexpectedElementError) Error() string {
	return fmt.Sprintf(`unexpected element %s at position %s`, u.Name, u.Position)
}

// Is checks if the given error matches the receiver.
func (u *UnexpectedElementError) Is(err error) bool {
	var o *UnexpectedElementError
	return errors.As(err, &o) && *o == *u
}

// UnexpectedEndElementError is returned when an end-element is found that does not match the currently open element.
type UnexpectedEndElementError struct {
	Position Position

	// Name is the element name.
	Name esixml.Name

	// Name is the expected end element name, if any.
	Expected esixml.Name
}

// Error returns a human-readable error message.
func (u *UnexpectedEndElementError) Error() string {
	if u.Expected.Local == "" {
		return fmt.Sprintf(`unexpected end element %s at position %s`, u.Name, u.Position)
	}

	return fmt.Sprintf(`unexpected end element %s at position %s, %s expected`, u.Name, u.Position, u.Expected)
}

// Is checks if the given error matches the receiver.
func (u *UnexpectedEndElementError) Is(err error) bool {
	var o *UnexpectedEndElementError
	return errors.As(err, &o) && *o == *u
}

// UnexpectedTokenError is returned when a specific token type was expected, but a different type was found.
type UnexpectedTokenError struct {
	Position Position

	// Type is the type of token that was encountered
	Type esixml.TokenType

	// Expected, if set, contains a list of expected token types.
	Expected []esixml.TokenType
}

// Error returns a human-readable error message.
func (u *UnexpectedTokenError) Error() string {
	return fmt.Sprintf(`unexpected token %s at position %s`, u.Type, u.Position)
}

// Is checks if the given error matches the receiver.
func (u *UnexpectedTokenError) Is(err error) bool {
	var o *UnexpectedTokenError
	return errors.As(err, &o) && o.Type == u.Type && slices.Equal(o.Expected, u.Expected)
}

// Node is the interface implemented by all ESI elements as well as [RawData].
type Node interface {
	// Pos returns the start and end position of the Node.
	Pos() (start, end int)

	node()
}

// Element is the interface implemented by all Node types that are based on ESI elements.
type Element interface {
	Node

	// Name returns the name of the element with the "esi" namespace.
	Name() esixml.Name
}

// Position is embedded into [Node] types and contains the start and end offsets of the node in the parsed input.
type Position = esixml.Position

// AttemptElement represents a <esi:attempt> element.
//
// See https://www.w3.org/TR/esi-lang/, 3.3 try | attempt | except.
type AttemptElement struct {
	Position Position

	// Attr contains all non-standard attributes specified on the element.
	Attr []esixml.Attr

	// Nodes contains all child nodes of the element.
	Nodes []Node
}

var _ Element = (*AttemptElement)(nil)

func (*AttemptElement) node() {}

// Name returns the element name.
func (e *AttemptElement) Name() esixml.Name {
	return esixml.Name{Space: "esi", Local: "attempt"}
}

// Pos returns the start and end position of the element.
func (e *AttemptElement) Pos() (start, end int) {
	return e.Position.Pos()
}

// ChooseElement represents a <esi:choose> element.
//
// See https://www.w3.org/TR/esi-lang/, 3.2 choose | when | otherwise.
type ChooseElement struct {
	Position Position

	// Attr contains all non-standard attributes specified on the element.
	Attr []esixml.Attr

	// When contains all <esi:when> nodes included in the element.
	When []*WhenElement

	// Otherwise contains the <esi:otherwise> element included in the element, if any.
	Otherwise *OtherwiseElement
}

var _ Element = (*ChooseElement)(nil)

func (*ChooseElement) node() {}

// Name returns the element name.
func (e *ChooseElement) Name() esixml.Name {
	return esixml.Name{Space: "esi", Local: "choose"}
}

// Pos returns the start and end position of the element.
func (e *ChooseElement) Pos() (start, end int) {
	return e.Position.Pos()
}

// CommentElement represents a <esi:comment> element.
//
// See https://www.w3.org/TR/esi-lang/, 3.4 comment.
type CommentElement struct {
	Position Position

	// Attr contains all non-standard attributes specified on the element.
	Attr []esixml.Attr

	// Text contains the comment text.
	Text string
}

var _ Element = (*CommentElement)(nil)

func (*CommentElement) node() {}

// Name returns the element name.
func (e *CommentElement) Name() esixml.Name {
	return esixml.Name{Space: "esi", Local: "comment"}
}

// Pos returns the start and end position of the element.
func (e *CommentElement) Pos() (start, end int) {
	return e.Position.Pos()
}

// ExceptElement represents a <esi:except> element.
//
// See https://www.w3.org/TR/esi-lang/, 3.3 try | attempt | except.
type ExceptElement struct {
	Position Position

	// Attr contains all non-standard attributes specified on the element.
	Attr []esixml.Attr

	// Nodes contains all child nodes of the element.
	Nodes []Node
}

var _ Element = (*ExceptElement)(nil)

func (*ExceptElement) node() {}

// Name returns the element name.
func (e *ExceptElement) Name() esixml.Name {
	return esixml.Name{Space: "esi", Local: "except"}
}

// Pos returns the start and end position of the element.
func (e *ExceptElement) Pos() (start, end int) {
	return e.Position.Pos()
}

// IncludeElement represents a <esi:include> element.
//
// See https://www.w3.org/TR/esi-lang/, 3.1 include.
type IncludeElement struct {
	Position Position

	// Attr contains all non-standard attributes specified on the element.
	Attr []esixml.Attr

	// Alt contains the alternative source that should be included, if the normal source is unavailable.
	Alt string

	// OnError contains the specified behaviour for errors.
	OnError ErrorBehaviour

	// Source contains the source that should be included.
	Source string
}

var _ Element = (*IncludeElement)(nil)

func (*IncludeElement) node() {}

// Name returns the element name.
func (e *IncludeElement) Name() esixml.Name {
	return esixml.Name{Space: "esi", Local: "include"}
}

// Pos returns the start and end position of the element.
func (e *IncludeElement) Pos() (start, end int) {
	return e.Position.Pos()
}

// InlineElement represents a <esi:inline> element.
//
// See https://www.w3.org/TR/esi-lang/, 3.2 inline.
type InlineElement struct {
	Position Position

	// Attr contains all non-standard attributes specified on the element.
	Attr []esixml.Attr

	// Name contains the name of the fragment.
	FragmentName string

	// Fragment is true if the fragment can be fetched.
	Fetchable bool

	// Nodes contains all child nodes of the element.
	Nodes []Node
}

var _ Element = (*InlineElement)(nil)

func (*InlineElement) node() {}

// Name returns the element name.
func (e *InlineElement) Name() esixml.Name {
	return esixml.Name{Space: "esi", Local: "inline"}
}

// Pos returns the start and end position of the element.
func (e *InlineElement) Pos() (start, end int) {
	return e.Position.Pos()
}

// OtherwiseElement represents a <esi:otherwise> element.
//
// See https://www.w3.org/TR/esi-lang/, 3.2 choose | when | otherwise.
type OtherwiseElement struct {
	Position Position

	// Attr contains all non-standard attributes specified on the element.
	Attr []esixml.Attr

	// Nodes contains all child nodes of the element.
	Nodes []Node
}

var _ Element = (*OtherwiseElement)(nil)

func (*OtherwiseElement) node() {}

// Name returns the element name.
func (e *OtherwiseElement) Name() esixml.Name {
	return esixml.Name{Space: "esi", Local: "otherwise"}
}

// Pos returns the start and end position of the element.
func (e *OtherwiseElement) Pos() (start, end int) {
	return e.Position.Pos()
}

// RawData represents raw, unprocessed data.
//
// The data may contain anything, including valid ESI elements.
type RawData struct {
	Position Position

	// Bytes contains the unprocessed data.
	Bytes []byte
}

func (*RawData) node() {}

// Pos returns the position.
func (r *RawData) Pos() (start, end int) {
	return r.Position.Pos()
}

// RemoveElement represents a <esi:remove> element.
//
// See https://www.w3.org/TR/esi-lang/, 3.5 remove.
type RemoveElement struct {
	Position Position

	// Attr contains all non-standard attributes specified on the element.
	Attr []esixml.Attr

	// Nodes contains all child nodes of the element.
	Nodes []Node
}

var _ Element = (*RemoveElement)(nil)

func (*RemoveElement) node() {}

// Name returns the element name.
func (e *RemoveElement) Name() esixml.Name {
	return esixml.Name{Space: "esi", Local: "remove"}
}

// Pos returns the start and end position of the element.
func (e *RemoveElement) Pos() (start, end int) {
	return e.Position.Pos()
}

// TryElement represents a <esi:try> element.
//
// See https://www.w3.org/TR/esi-lang/, 3.3 try | attempt | except.
type TryElement struct {
	Position Position

	// Attr contains all non-standard attributes specified on the element.
	Attr []esixml.Attr

	// Attempt contains the nodes that should be attempted to be rendered.
	Attempt *AttemptElement // attempt

	// Except contains the nodes that should be rendered if the initial attempt failed.
	Except *ExceptElement // except
}

var _ Element = (*TryElement)(nil)

func (*TryElement) node() {}

// Name returns the element name.
func (e *TryElement) Name() esixml.Name {
	return esixml.Name{Space: "esi", Local: "try"}
}

// Pos returns the start and end position of the element.
func (e *TryElement) Pos() (start, end int) {
	return e.Position.Pos()
}

// VarsElement represents a <esi:vars> element.
//
// See https://www.w3.org/TR/esi-lang/, 3.6 vars.
type VarsElement struct {
	Position Position

	// Attr contains all non-standard attributes specified on the element.
	Attr []esixml.Attr

	// Nodes contains all child nodes of the element.
	Nodes []Node
}

var _ Element = (*VarsElement)(nil)

func (*VarsElement) node() {}

// Name returns the element name.
func (e *VarsElement) Name() esixml.Name {
	return esixml.Name{Space: "esi", Local: "vars"}
}

// Pos returns the start and end position of the element.
func (e *VarsElement) Pos() (start, end int) {
	return e.Position.Pos()
}

// WhenElement represents a <esi:when> element.
//
// See https://www.w3.org/TR/esi-lang/, 3.2 choose | when | otherwise.
type WhenElement struct {
	Position Position

	// Attr contains all non-standard attributes specified on the element.
	Attr []esixml.Attr

	// Test contains the condition for the element.
	Test string

	// Nodes contains all child nodes of the element.
	Nodes []Node
}

var _ Element = (*WhenElement)(nil)

func (*WhenElement) node() {}

// Name returns the element name.
func (e *WhenElement) Name() esixml.Name {
	return esixml.Name{Space: "esi", Local: "when"}
}

// Pos returns the start and end position of the element.
func (e *WhenElement) Pos() (start, end int) {
	return e.Position.Pos()
}

// Parser implements parsing of documents containing ESI instructions, returning the parsed elements and the unprocessed
// data.
type Parser struct {
	reader      esixml.Reader
	unreadToken esixml.Token
	err         error

	// current stack. nil values mark the start of a scope.
	stack []Node

	stateFn func(*Parser) (Node, error)
}

// NewParser returns a new Parser set to read from in.
//
// This is a shorthand for creating a new [Parser] and calling [Parser.Reset] on it.
func NewParser(in io.Reader) *Parser {
	p := &Parser{}
	p.Reset(in)
	return p
}

// All yields all remaining nodes from the parser.
func (p *Parser) All(yield func(Node, error) bool) {
	for {
		node, err := p.Next()

		if errors.Is(err, io.EOF) {
			return
		}

		if !yield(node, err) {
			return
		}

		if err != nil {
			return
		}
	}
}

// Next returns the next Node if any.
//
// If an error occurred, future calls till return the same error.
//
// After all data was read, if there were no previous errors, Next will return [io.EOF].
func (p *Parser) Next() (Node, error) {
	var node Node

	for p.err == nil {
		node, p.err = p.stateFn(p)

		if node != nil {
			return node, nil
		}
	}

	if !errors.Is(p.err, io.EOF) {
		return nil, p.err
	}

	if el := p.currentScope(); el != nil {
		start, end := el.Pos()

		return nil, &UnclosedElementError{
			Position: Position{
				Start: start,
				End:   end,
			},
			Name: el.(Element).Name(),
		}
	}

	return nil, io.EOF
}

// Reset resets the Parser to read from in.
//
// This allows re-using the parser for different inputs.
func (p *Parser) Reset(in io.Reader) {
	if len(p.stack) == 0 || cap(p.stack) > 32 {
		p.stack = make([]Node, 0, 32)
	} else {
		clear(p.stack)
	}

	p.err = nil
	p.stack = p.stack[:0]
	p.unreadToken = esixml.Token{}
	p.stateFn = (*Parser).parseDataOrElement
	p.reader.Reset(in)
}

func (p *Parser) nextToken() (esixml.Token, error) {
	if p.unreadToken.Type != esixml.TokenTypeInvalid {
		t := p.unreadToken
		p.unreadToken = esixml.Token{}
		return t, nil
	}

	return p.reader.Next()
}

func (p *Parser) mustNextToken() (esixml.Token, error) {
	tok, err := p.nextToken()
	if err != nil {
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}
		return esixml.Token{}, err
	}
	return tok, nil
}

func (p *Parser) mustNextTyped(typ esixml.TokenType) (esixml.Token, error) {
	tok, err := p.mustNextToken()
	if err != nil {
		return esixml.Token{}, err
	}
	if tok.Type != typ {
		return esixml.Token{}, &UnexpectedTokenError{
			Position: tok.Position,
			Type:     tok.Type,
			Expected: []esixml.TokenType{typ},
		}
	}
	return tok, nil
}

func (p *Parser) mustNextEndElement(localName string) (esixml.Token, error) {
	tok, err := p.mustNextTyped(esixml.TokenTypeEndElement)
	if err != nil {
		return esixml.Token{}, err
	}
	if tok.Name.Local != localName {
		return esixml.Token{}, &UnexpectedEndElementError{
			Position: tok.Position,
			Name:     tok.Name,
		}
	}
	return tok, nil
}

func (p *Parser) mustNextStartElement(localName string) (esixml.Token, error) {
	tok, err := p.mustNextTyped(esixml.TokenTypeStartElement)
	if err != nil {
		return esixml.Token{}, err
	}
	if tok.Name.Local != localName {
		return esixml.Token{}, &UnexpectedElementError{
			Position: tok.Position,
			Name:     tok.Name,
		}
	}
	return tok, nil
}

func (p *Parser) current() Node {
	if len(p.stack) == 0 {
		return nil
	}
	return p.stack[len(p.stack)-1]
}

func (p *Parser) currentScope() Node {
	for i := len(p.stack) - 1; i >= 1; i-- {
		if p.stack[i] != nil {
			continue
		}
		return p.stack[i-1]
	}

	return nil
}

func (p *Parser) exitScope() []Node {
	for i := len(p.stack) - 1; i >= 0; i-- {
		if p.stack[i] != nil {
			continue
		}

		var nodes []Node
		p.stack, nodes = p.stack[:i], p.stack[i+1:]

		if len(nodes) == 0 {
			return nil
		}

		return slices.Clone(nodes)
	}

	panic("no open scope")
}

func (p *Parser) popIfRoot() Node {
	if len(p.stack) != 1 {
		return nil
	}
	node := p.stack[0]
	p.stack = p.stack[:0]
	return node
}

func (p *Parser) push(node Node) {
	if node == nil {
		panic("tried to push nil node")
	}
	p.stack = append(p.stack, node)
}

func (p *Parser) pushScope(node Node) {
	if node == nil {
		panic("tried to push nil node for scope")
	}
	p.stack = append(p.stack, node, nil)
}

func (p *Parser) pushNestedOrReturn(node Node) Node {
	if len(p.stack) == 0 {
		return node
	}

	p.push(node)

	return nil
}

func takeAttr(attrs *[]esixml.Attr, name string) (esixml.Attr, bool) {
	for i, attr := range *attrs {
		if attr.Name.Space != "" || attr.Name.Local != name {
			continue
		}

		// Do not keep the backing array if not needed anymore
		if len(*attrs) == 1 {
			*attrs = nil
		} else {
			*attrs = slices.Delete(*attrs, i, i+1)
		}

		return attr, true
	}
	return esixml.Attr{}, false
}

func (p *Parser) parseAttemptElement() (Node, error) {
	tok, err := p.mustNextStartElement("attempt")
	if err != nil {
		return nil, err
	}

	if _, ok := p.currentScope().(*TryElement); !ok {
		return nil, &UnexpectedElementError{Position: tok.Position, Name: tok.Name}
	}

	p.pushScope(&AttemptElement{Position: tok.Position, Attr: tok.Attr})
	p.stateFn = (*Parser).parseDataOrElement
	return nil, nil
}

func (p *Parser) parseAttemptElementEnd() (Node, error) {
	tok, err := p.mustNextEndElement("attempt")
	if err != nil {
		return nil, err
	}

	children := p.exitScope()

	el := p.current().(*AttemptElement)
	el.Nodes = children
	el.Position.End = tok.Position.End

	p.stateFn = (*Parser).parseDataOrElement
	return nil, nil
}

func (p *Parser) parseChooseElement() (Node, error) {
	tok, err := p.mustNextStartElement("choose")
	if err != nil {
		return nil, err
	}

	if tok.Closed {
		return nil, &EmptyElementError{Position: tok.Position, Name: tok.Name}
	}

	p.pushScope(&ChooseElement{Position: tok.Position, Attr: tok.Attr})
	p.stateFn = (*Parser).parseDataOrElement
	return nil, nil
}

func (p *Parser) parseChooseElementEnd() (Node, error) {
	tok, err := p.mustNextEndElement("choose")
	if err != nil {
		return nil, err
	}

	children := p.exitScope()

	el := p.current().(*ChooseElement)
	el.Position.End = tok.Position.End

	for _, node := range children {
		switch v := node.(type) {
		case *WhenElement:
			el.When = append(el.When, v)
		case *OtherwiseElement:
			if el.Otherwise != nil {
				return nil, &DuplicateElementError{Position: v.Position, Name: v.Name()}
			}
			el.Otherwise = v
		default:
			// ignore other data, as per spec
		}
	}

	if len(el.When) == 0 {
		return nil, &MissingElementError{Position: tok.Position, Name: esixml.Name{Space: "esi", Local: "when"}}
	}

	p.stateFn = (*Parser).parseDataOrElement
	return p.popIfRoot(), nil
}

func (p *Parser) parseCommentElement() (Node, error) {
	tok, err := p.mustNextStartElement("comment")
	if err != nil {
		return nil, err
	}

	if !tok.Closed {
		return nil, &UnclosedElementError{Position: tok.Position, Name: tok.Name}
	}

	text, ok := takeAttr(&tok.Attr, "text")
	if !ok {
		return nil, &MissingAttributeError{
			Position:  tok.Position,
			Element:   tok.Name,
			Attribute: esixml.Name{Local: "text"},
		}
	}

	p.stateFn = (*Parser).parseDataOrElement

	return p.pushNestedOrReturn(&CommentElement{
		Position: tok.Position,
		Attr:     tok.Attr,
		Text:     text.Value,
	}), nil
}

func (p *Parser) parseData() (Node, error) {
	tok, err := p.mustNextTyped(esixml.TokenTypeData)
	if err != nil {
		return nil, err
	}

	p.stateFn = (*Parser).parseDataOrElement

	return p.pushNestedOrReturn(&RawData{
		Position: tok.Position,
		Bytes:    tok.Data,
	}), nil
}

func (p *Parser) parseDataOrElement() (Node, error) {
	tok, err := p.nextToken()
	if err != nil {
		return nil, err
	}

	switch tok.Type {
	case esixml.TokenTypeInvalid:
		panic("unreachable")
	case esixml.TokenTypeStartElement:
		p.stateFn = (*Parser).parseElement
	case esixml.TokenTypeEndElement:
		p.stateFn = (*Parser).parseEndElement
	case esixml.TokenTypeData:
		p.stateFn = (*Parser).parseData
	}

	p.unreadToken = tok
	return nil, nil
}

func (p *Parser) parseElement() (Node, error) {
	tok, err := p.mustNextTyped(esixml.TokenTypeStartElement)
	if err != nil {
		return nil, err
	}

	switch tok.Name.Local {
	case "attempt":
		p.stateFn = (*Parser).parseAttemptElement
	case "choose":
		p.stateFn = (*Parser).parseChooseElement
	case "comment":
		p.stateFn = (*Parser).parseCommentElement
	case "except":
		p.stateFn = (*Parser).parseExceptElement
	case "include":
		p.stateFn = (*Parser).parseIncludeElement
	case "inline":
		p.stateFn = (*Parser).parseInlineElement
	case "otherwise":
		p.stateFn = (*Parser).parseOtherwiseElement
	case "remove":
		p.stateFn = (*Parser).parseRemoveElement
	case "try":
		p.stateFn = (*Parser).parseTryElement
	case "vars":
		p.stateFn = (*Parser).parseVarsElement
	case "when":
		p.stateFn = (*Parser).parseWhenElement
	default:
		return nil, &InvalidElementError{Position: tok.Position, Name: tok.Name}
	}

	p.unreadToken = tok
	return nil, nil
}

func (p *Parser) parseEndElement() (Node, error) {
	tok, err := p.mustNextTyped(esixml.TokenTypeEndElement)
	if err != nil {
		return nil, err
	}

	parent, _ := p.currentScope().(Element)
	if parent == nil {
		return nil, &UnexpectedEndElementError{Position: tok.Position, Name: tok.Name}
	}
	if parent.Name() != tok.Name {
		return nil, &UnexpectedEndElementError{Position: tok.Position, Name: tok.Name, Expected: parent.Name()}
	}

	switch tok.Name.Local {
	case "attempt":
		p.stateFn = (*Parser).parseAttemptElementEnd
	case "choose":
		p.stateFn = (*Parser).parseChooseElementEnd
	case "except":
		p.stateFn = (*Parser).parseExceptElementEnd
	case "inline":
		p.stateFn = (*Parser).parseInlineElementEnd
	case "otherwise":
		p.stateFn = (*Parser).parseOtherwiseElementEnd
	case "remove":
		p.stateFn = (*Parser).parseRemoveElementEnd
	case "try":
		p.stateFn = (*Parser).parseTryElementEnd
	case "vars":
		p.stateFn = (*Parser).parseVarsElementEnd
	case "when":
		p.stateFn = (*Parser).parseWhenElementEnd
	default:
		return nil, &InvalidElementError{Position: tok.Position, Name: tok.Name}
	}

	p.unreadToken = tok
	return nil, nil
}

func (p *Parser) parseExceptElement() (Node, error) {
	tok, err := p.mustNextStartElement("except")
	if err != nil {
		return nil, err
	}

	if _, ok := p.currentScope().(*TryElement); !ok {
		return nil, &UnexpectedElementError{Position: tok.Position, Name: tok.Name}
	}

	p.pushScope(&ExceptElement{Position: tok.Position, Attr: tok.Attr})
	p.stateFn = (*Parser).parseDataOrElement
	return nil, nil
}

func (p *Parser) parseExceptElementEnd() (Node, error) {
	tok, err := p.mustNextEndElement("except")
	if err != nil {
		return nil, err
	}

	children := p.exitScope()

	el := p.current().(*ExceptElement)
	el.Nodes = children
	el.Position.End = tok.Position.End

	p.stateFn = (*Parser).parseDataOrElement
	return nil, nil
}

func (p *Parser) parseIncludeElement() (Node, error) {
	tok, err := p.mustNextStartElement("include")
	if err != nil {
		return nil, err
	}

	if !tok.Closed {
		return nil, &UnclosedElementError{Position: tok.Position, Name: tok.Name}
	}

	alt, _ := takeAttr(&tok.Attr, "alt")

	onError, ok := takeAttr(&tok.Attr, "onerror")
	if ok && onError.Value != string(ErrorBehaviourContinue) {
		return nil, &InvalidAttributeValueError{
			Position: Position{Start: onError.Position.Start, End: onError.Position.End},
			Element:  tok.Name,
			Name:     esixml.Name{Local: "onerror"},
			Value:    onError.Value,
			Allowed: []string{
				string(ErrorBehaviourContinue),
			},
		}
	}

	src, ok := takeAttr(&tok.Attr, "src")
	if !ok {
		return nil, &MissingAttributeError{Position: tok.Position, Element: tok.Name, Attribute: esixml.Name{Local: "src"}}
	}

	p.stateFn = (*Parser).parseDataOrElement

	return p.pushNestedOrReturn(&IncludeElement{
		Position: tok.Position,
		Attr:     tok.Attr,
		Alt:      alt.Value,
		OnError:  ErrorBehaviour(onError.Value),
		Source:   src.Value,
	}), nil
}

func (p *Parser) parseInlineElement() (Node, error) {
	tok, err := p.mustNextStartElement("inline")
	if err != nil {
		return nil, err
	}

	name, ok := takeAttr(&tok.Attr, "name")
	if !ok {
		return nil, &MissingAttributeError{
			Position:  tok.Position,
			Element:   tok.Name,
			Attribute: esixml.Name{Local: "name"},
		}
	}

	fetchable, ok := takeAttr(&tok.Attr, "fetchable")
	if !ok {
		return nil, &MissingAttributeError{
			Position:  tok.Position,
			Element:   tok.Name,
			Attribute: esixml.Name{Local: "fetchable"},
		}
	}

	switch fetchable.Value {
	case "no", "yes":
	default:
		return nil, &InvalidAttributeValueError{
			Position: tok.Position,
			Element:  tok.Name,
			Name:     esixml.Name{Local: "fetchable"},
			Value:    fetchable.Value,
			Allowed:  []string{"no", "yes"},
		}
	}

	if tok.Closed {
		return nil, &EmptyElementError{Position: tok.Position, Name: tok.Name}
	}

	p.pushScope(&InlineElement{
		Position:     tok.Position,
		Attr:         tok.Attr,
		FragmentName: name.Value,
		Fetchable:    fetchable.Value == "yes",
	})
	p.stateFn = (*Parser).parseDataOrElement
	return nil, nil
}

func (p *Parser) parseInlineElementEnd() (Node, error) {
	tok, err := p.mustNextEndElement("inline")
	if err != nil {
		return nil, err
	}

	children := p.exitScope()

	el := p.current().(*InlineElement)
	el.Nodes = children
	el.Position.End = tok.Position.End

	p.stateFn = (*Parser).parseDataOrElement
	return p.popIfRoot(), nil
}

func (p *Parser) parseOtherwiseElement() (Node, error) {
	tok, err := p.mustNextStartElement("otherwise")
	if err != nil {
		return nil, err
	}

	if _, ok := p.currentScope().(*ChooseElement); !ok {
		return nil, &UnexpectedElementError{Position: tok.Position, Name: tok.Name}
	}

	p.pushScope(&OtherwiseElement{Position: tok.Position, Attr: tok.Attr})
	p.stateFn = (*Parser).parseDataOrElement
	return nil, nil
}

func (p *Parser) parseOtherwiseElementEnd() (Node, error) {
	tok, err := p.mustNextEndElement("otherwise")
	if err != nil {
		return nil, err
	}

	children := p.exitScope()

	el := p.current().(*OtherwiseElement)
	el.Nodes = children
	el.Position.End = tok.Position.End

	p.stateFn = (*Parser).parseDataOrElement
	return nil, nil
}

func (p *Parser) parseRemoveElement() (Node, error) {
	tok, err := p.mustNextStartElement("remove")
	if err != nil {
		return nil, err
	}

	if tok.Closed {
		return nil, &EmptyElementError{Position: tok.Position, Name: tok.Name}
	}

	e := &RemoveElement{Position: tok.Position, Attr: tok.Attr}

	p.pushScope(e)
	p.stateFn = (*Parser).parseDataOrElement
	return nil, nil
}

func (p *Parser) parseRemoveElementEnd() (Node, error) {
	tok, err := p.mustNextEndElement("remove")
	if err != nil {
		return nil, err
	}

	children := p.exitScope()

	el := p.current().(*RemoveElement)
	el.Nodes = children
	el.Position.End = tok.Position.End

	p.stateFn = (*Parser).parseDataOrElement
	return p.popIfRoot(), nil
}

func (p *Parser) parseTryElement() (Node, error) {
	tok, err := p.mustNextStartElement("try")
	if err != nil {
		return nil, err
	}

	if tok.Closed {
		return nil, &EmptyElementError{Position: tok.Position, Name: tok.Name}
	}

	p.pushScope(&TryElement{Position: tok.Position, Attr: tok.Attr})
	p.stateFn = (*Parser).parseDataOrElement
	return nil, nil
}

func (p *Parser) parseTryElementEnd() (Node, error) {
	tok, err := p.mustNextEndElement("try")
	if err != nil {
		return nil, err
	}

	children := p.exitScope()

	el := p.current().(*TryElement)
	el.Position.End = tok.Position.End

	for _, node := range children {
		switch v := node.(type) {
		case *AttemptElement:
			if el.Attempt != nil {
				return nil, &DuplicateElementError{Position: v.Position, Name: v.Name()}
			}
			el.Attempt = v
		case *ExceptElement:
			if el.Except != nil {
				return nil, &DuplicateElementError{Position: v.Position, Name: v.Name()}
			}
			el.Except = v
		default:
			// ignore other data, as per spec
		}
	}

	if el.Attempt == nil {
		return nil, &MissingElementError{Position: tok.Position, Name: esixml.Name{Space: "esi", Local: "attempt"}}
	}

	if el.Except == nil {
		return nil, &MissingElementError{Position: tok.Position, Name: esixml.Name{Space: "esi", Local: "except"}}
	}

	p.stateFn = (*Parser).parseDataOrElement
	return p.popIfRoot(), nil
}

func (p *Parser) parseVarsElement() (Node, error) {
	tok, err := p.mustNextStartElement("vars")
	if err != nil {
		return nil, err
	}

	if tok.Closed {
		return nil, &EmptyElementError{Position: tok.Position, Name: tok.Name}
	}

	e := &VarsElement{Position: tok.Position, Attr: tok.Attr}

	p.pushScope(e)
	p.stateFn = (*Parser).parseDataOrElement
	return nil, nil
}

func (p *Parser) parseVarsElementEnd() (Node, error) {
	tok, err := p.mustNextEndElement("vars")
	if err != nil {
		return nil, err
	}

	children := p.exitScope()

	el := p.current().(*VarsElement)
	el.Nodes = children
	el.Position.End = tok.Position.End

	p.stateFn = (*Parser).parseDataOrElement
	return p.popIfRoot(), nil
}

func (p *Parser) parseWhenElement() (Node, error) {
	tok, err := p.mustNextStartElement("when")
	if err != nil {
		return nil, err
	}

	if _, ok := p.currentScope().(*ChooseElement); !ok {
		return nil, &UnexpectedElementError{Position: tok.Position, Name: tok.Name}
	}

	test, ok := takeAttr(&tok.Attr, "test")
	if !ok {
		return nil, &MissingAttributeError{Position: tok.Position, Element: tok.Name, Attribute: esixml.Name{Local: "test"}}
	}

	p.pushScope(&WhenElement{Position: tok.Position, Attr: tok.Attr, Test: test.Value})
	p.stateFn = (*Parser).parseDataOrElement
	return nil, nil
}

func (p *Parser) parseWhenElementEnd() (Node, error) {
	tok, err := p.mustNextEndElement("when")
	if err != nil {
		return nil, err
	}

	children := p.exitScope()

	el := p.current().(*WhenElement)
	el.Nodes = children
	el.Position.End = tok.Position.End

	p.stateFn = (*Parser).parseDataOrElement
	return nil, nil
}
