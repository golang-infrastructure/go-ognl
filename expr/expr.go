package expr

import (
	"errors"
	"github/songzhibin97/go-ognl/token"
)

var (
	ErrLength          = errors.New("invalid length")
	ErrFirstIdentifier = errors.New("first identifier must be '.' or '['")
	ErrLastIdentifier  = errors.New("last identifier must be ']' or 'string' or 'len' or 'int'")
	ErrPeriodBack      = errors.New("'.' must be behind 'string' or 'len' or 'int'")
	ErrStringBack      = errors.New("'string' must be behind '.' or '[' or 'int")
	ErrIntBack         = errors.New("'int' must be behind ']' or '.'")
	ErrLBrackBack      = errors.New("'[' must be behind 'int'")
	ErrRBrackBack      = errors.New("']' must be behind '.' or '[' or 'len'")
	ErrLenBack         = errors.New("'len' must be behind 'EOF'")
)

func ParseToken(tokens []*token.Token) ([]*token.Token, error) {
	ln := len(tokens)
	if ln == 0 {
		return nil, ErrLength
	}

	if tokens[0].Type != token.PERIOD && tokens[0].Type != token.LBRACK {
		return nil, ErrFirstIdentifier
	}
	if tokens[ln-1].Type != token.STRING && tokens[ln-1].Type != token.RBRACK && tokens[ln-1].Type != token.Len && tokens[ln-1].Type != token.INT {
		return nil, ErrLastIdentifier
	}

	filter := make([]*token.Token, 0, len(tokens))
	for i := 0; i < ln; i++ {
		switch tokens[i].Type {
		case token.PERIOD:
			if i+1 >= ln || (tokens[i+1].Type != token.STRING && tokens[i+1].Type != token.Len && tokens[i+1].Type != token.INT) {
				return nil, ErrPeriodBack
			}
			continue
		case token.STRING:
			if i+1 < ln && (tokens[i+1].Type != token.PERIOD && tokens[i+1].Type != token.LBRACK && tokens[i+1].Type != token.INT) {
				return nil, ErrStringBack
			}
		case token.INT:
			if i+1 < ln && (tokens[i+1].Type != token.RBRACK && tokens[i+1].Type != token.PERIOD) {
				return nil, ErrIntBack
			}
		case token.LBRACK:
			if i+1 >= ln || (tokens[i+1].Type != token.INT) {
				return nil, ErrLBrackBack
			}
			continue
		case token.RBRACK:
			if i+1 < ln && (tokens[i+1].Type != token.PERIOD && tokens[i+1].Type != token.LBRACK && tokens[i+1].Type != token.Len) {
				return nil, ErrRBrackBack
			}
			continue
		case token.Len:
			if i+1 != ln {
				return nil, ErrLenBack
			}
		default:
			continue
		}
		filter = append(filter, tokens[i])
	}
	return filter, nil
}
