package steam

import "fmt"

// Value is either a quoted scalar or an object containing named values.
type Value struct {
	Scalar string
	object map[string]Value
}

// Parse parses the quoted-string and nested-object subset of Steam KeyValues.
func Parse(data []byte) (Value, error) {
	parser := parser{lexer: lexer{data: data}}
	object, err := parser.parseObject(false)
	if err != nil {
		return Value{}, err
	}
	return Value{object: object}, nil
}

// Object returns the named object value.
func (v Value) Object(key string) (Value, bool) {
	value, ok := v.object[key]
	if !ok || value.object == nil {
		return Value{}, false
	}
	return value, true
}

// String returns the named scalar value.
func (v Value) String(key string) (string, bool) {
	value, ok := v.object[key]
	if !ok || value.object != nil {
		return "", false
	}
	return value.Scalar, true
}

// Manifest contains the Steam app manifest fields needed for discovery.
type Manifest struct {
	AppID      string
	Name       string
	InstallDir string
}

// ParseManifest parses and validates the required fields in an AppState object.
func ParseManifest(data []byte) (Manifest, error) {
	root, err := Parse(data)
	if err != nil {
		return Manifest{}, fmt.Errorf("parse app manifest: %w", err)
	}
	appState, ok := root.Object("AppState")
	if !ok {
		return Manifest{}, fmt.Errorf("app manifest: AppState object missing")
	}

	appID, ok := appState.String("appid")
	if !ok || appID == "" {
		return Manifest{}, fmt.Errorf("app manifest: appid missing or empty")
	}
	name, ok := appState.String("name")
	if !ok || name == "" {
		return Manifest{}, fmt.Errorf("app manifest: name missing or empty")
	}
	installDir, ok := appState.String("installdir")
	if !ok || installDir == "" {
		return Manifest{}, fmt.Errorf("app manifest: installdir missing or empty")
	}

	return Manifest{AppID: appID, Name: name, InstallDir: installDir}, nil
}

type tokenKind uint8

const (
	tokenEOF tokenKind = iota
	tokenString
	tokenOpenBrace
	tokenCloseBrace
)

type token struct {
	kind   tokenKind
	text   string
	offset int
}

type lexer struct {
	data []byte
	pos  int
}

func (l *lexer) next() (token, error) {
	l.skipIgnored()
	if l.pos == len(l.data) {
		return token{kind: tokenEOF, offset: l.pos}, nil
	}

	offset := l.pos
	switch l.data[l.pos] {
	case '{':
		l.pos++
		return token{kind: tokenOpenBrace, offset: offset}, nil
	case '}':
		l.pos++
		return token{kind: tokenCloseBrace, offset: offset}, nil
	case '"':
		return l.quoted()
	default:
		return token{}, fmt.Errorf("unsupported unquoted token at byte %d", offset)
	}
}

func (l *lexer) skipIgnored() {
	for {
		for l.pos < len(l.data) && isASCIIWhitespace(l.data[l.pos]) {
			l.pos++
		}
		if l.pos+1 >= len(l.data) || l.data[l.pos] != '/' || l.data[l.pos+1] != '/' {
			return
		}
		l.pos += 2
		for l.pos < len(l.data) && l.data[l.pos] != '\n' {
			l.pos++
		}
	}
}

func (l *lexer) quoted() (token, error) {
	offset := l.pos
	l.pos++
	value := make([]byte, 0)
	for l.pos < len(l.data) {
		current := l.data[l.pos]
		l.pos++
		switch current {
		case '"':
			return token{kind: tokenString, text: string(value), offset: offset}, nil
		case '\\':
			if l.pos == len(l.data) {
				return token{}, fmt.Errorf("unexpected EOF in quoted token at byte %d", offset)
			}
			escaped := l.data[l.pos]
			l.pos++
			if escaped != '"' && escaped != '\\' {
				return token{}, fmt.Errorf("unsupported escape \\%c at byte %d", escaped, l.pos-2)
			}
			value = append(value, escaped)
		default:
			value = append(value, current)
		}
	}
	return token{}, fmt.Errorf("unexpected EOF in quoted token at byte %d", offset)
}

func isASCIIWhitespace(value byte) bool {
	switch value {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return false
	}
}

type parser struct {
	lexer lexer
}

func (p *parser) parseObject(expectClose bool) (map[string]Value, error) {
	object := make(map[string]Value)
	for {
		key, err := p.lexer.next()
		if err != nil {
			return nil, err
		}
		switch key.kind {
		case tokenEOF:
			if expectClose {
				return nil, fmt.Errorf("unexpected EOF: object is not closed")
			}
			return object, nil
		case tokenCloseBrace:
			if !expectClose {
				return nil, fmt.Errorf("unmatched closing brace at byte %d", key.offset)
			}
			return object, nil
		case tokenOpenBrace:
			return nil, fmt.Errorf("expected quoted key at byte %d, found opening brace", key.offset)
		}

		value, err := p.lexer.next()
		if err != nil {
			return nil, err
		}
		switch value.kind {
		case tokenString:
			object[key.text] = Value{Scalar: value.text}
		case tokenOpenBrace:
			nested, err := p.parseObject(true)
			if err != nil {
				return nil, err
			}
			object[key.text] = Value{object: nested}
		case tokenEOF:
			return nil, fmt.Errorf("missing value for key %q: unexpected EOF", key.text)
		case tokenCloseBrace:
			return nil, fmt.Errorf("missing value for key %q before closing brace at byte %d", key.text, value.offset)
		}
	}
}
