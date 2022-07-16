package token

import (
	"strconv"
)

type Token struct {
	Type  Type
	Value string
}

func NewToken(tp Type, value string) *Token {
	return &Token{
		Type:  tp,
		Value: value,
	}
}

type Type int

func (tok Type) String() string {
	s := ""
	if 0 <= tok && tok < Type(len(tokens)) {
		s = tokens[tok]
	}
	if s == "" {
		s = "token(" + strconv.Itoa(int(tok)) + ")"
	}
	return s
}

const (
	EOF Type = iota
	STRING
	INT
	UNKNOWN
	keyword_beg
	Len    // len
	PERIOD // .
	LBRACK // [
	RBRACK // ]
	keyword_end
)

var tokens = [...]string{
	PERIOD: ".",
	LBRACK: "[",
	RBRACK: "]",
	STRING: "string",
	INT:    "int",
	Len:    "len",
}

// keywords 关键字
var keywords map[string]Type

func init() {
	// 预处理,将关键字存入keywords map
	keywords = make(map[string]Type)
	for i := keyword_beg + 1; i < keyword_end; i++ {
		keywords[tokens[i]] = i
	}
}

func LookupOf(ident string) Type {
	if tok, is_keyword := keywords[ident]; is_keyword {
		return tok
	}
	return STRING
}
