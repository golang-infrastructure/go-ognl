package lexer

import (
	"github/songzhibin97/go-ognl/token"
	"unicode"
)

type Lexer struct {
	pos   int    // 解析器当前解析到的位置
	ln    int    // input 长度
	input []rune // 解析器需要解析的字符串
}

// next
// @Description: 获取下一个字符,将其pos移动到下一位
// @receiver l
// @return rune
func (l *Lexer) next() rune {
	if l.pos >= l.ln {
		// 0 => EOF
		return 0
	}

	ret := l.input[l.pos]
	l.pos++
	return ret
}

// peek
// @Description: 获取下一个字符,但不移动pos
// @param l:
// @param offset: 偏移量
// @return rune
func (l *Lexer) peek(offset int) rune {
	if l.pos+offset >= l.ln {
		return 0
	}
	return l.input[l.pos+offset]
}

func isLetter(v rune, index int) bool {
	return unicode.IsLetter(v) && v != '_' && (index == 0 || !unicode.IsDigit(v))
}

func isDigit(v rune) bool {
	return unicode.IsDigit(v)
}

func (l *Lexer) letter() string {
	pos := l.pos
	for ; l.pos < l.ln; l.pos++ {
		v := l.input[l.pos]
		if !isLetter(v, pos-l.pos) {
			break
		}
	}
	ret := l.input[pos:l.pos]
	return string(ret)
}

func (l *Lexer) digit() string {
	pos := l.pos
	for ; l.pos < l.ln; l.pos++ {
		v := l.input[l.pos]
		if !isDigit(v) {
			break
		}
	}
	ret := l.input[pos:l.pos]
	return string(ret)
}

func (l *Lexer) skipInterference() {
	for ; l.pos < l.ln; l.pos++ {
		switch l.input[l.pos] {
		case ' ':
		case '\n':
		case '\r':
		case '\t':
		default:
			return
		}
	}
}

// NextToken
// @Description: 解析获取下一个有效的 Token
// @receiver l
// @return *token.Token
func (l *Lexer) NextToken() *token.Token {
	var tk *token.Token
	l.skipInterference()
	v := l.next()
	switch v {
	case 0:
		tk = token.NewToken(token.EOF, "")
	case '[':
		tk = token.NewToken(token.LBRACK, "[")
	case ']':
		tk = token.NewToken(token.RBRACK, "]")
	case '.':
		tk = token.NewToken(token.PERIOD, ".")
	default:
		switch {
		case isLetter(v, 0):
			identifier := string(v) + l.letter()
			tk = token.NewToken(token.LookupOf(identifier), identifier)
		case isDigit(v):
			tk = token.NewToken(token.INT, string(v)+l.digit())
		default:
			tk = token.NewToken(token.UNKNOWN, "")
		}
	}
	return tk
}

// NewLexer
// @Description: 创建新词法解析器
// @param input:
// @return *Lexer
func NewLexer(input string) *Lexer {
	v := &Lexer{
		input: []rune(input),
	}
	v.ln = len(v.input)
	return v
}

func (l *Lexer) GetToken() []*token.Token {
	var ret []*token.Token
	for tok := l.NextToken(); tok.Type != token.EOF; tok = l.NextToken() {
		if tok.Type == token.UNKNOWN {
			continue
		}
		ret = append(ret, tok)
	}
	return ret
}
