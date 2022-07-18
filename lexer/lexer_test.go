package lexer

import (
	"github.com/songzhibin97/go-ognl/token"
	"testing"
)

func TestNewLexer(t *testing.T) {
	query := ".Name.List[10].Field"
	l := NewLexer(query)
	for {
		v := l.NextToken()
		if v.Type == token.EOF {
			break
		}
		if v.Type == token.UNKNOWN {
			continue
		}
		t.Log(v.Type, v.Value)
	}
}
