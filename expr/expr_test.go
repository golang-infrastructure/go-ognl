package expr

import (
	"errors"
	"github/songzhibin97/go-ognl/lexer"
	"testing"
)

func Test_parseToken(t *testing.T) {
	test := []struct {
		V   string
		Err error
	}{
		{"", ErrLength},
		{"]", ErrFirstIdentifier},
		{"10", ErrFirstIdentifier},
		{".A.", ErrLastIdentifier},
		{".A[", ErrLastIdentifier},
		{"..", ErrLastIdentifier},
		{".[", ErrLastIdentifier},
		{".]", ErrPeriodBack},
		{".Filed len", ErrStringBack},
		{".Filed]", ErrStringBack},
		{"[10[", ErrLastIdentifier},
		{"[10 len", ErrIntBack},
		{"[10 Filed", ErrIntBack},
		{"[]", ErrLBrackBack},
		{"[[", ErrLastIdentifier},
		{"[ Field", ErrLBrackBack},
		{"[ len", ErrLBrackBack},
		{"[10]Filed", ErrRBrackBack},
		{"[10]]", ErrRBrackBack},
		{".len[", ErrLastIdentifier},
		{".len]", ErrLenBack},
		{".len Field", ErrLenBack},
		{".len.", ErrLastIdentifier},
		{".File 10", nil},
		{"[10][20].Filed 10.Test", nil},
		{".len", nil},
	}
	for index, v := range test {
		_, err := ParseToken(lexer.NewLexer(v.V).GetToken())
		if !errors.Is(err, v.Err) {
			t.Fatalf("index=%d expected=%q, got=%q", index, err, v.Err)
		}
	}
}
