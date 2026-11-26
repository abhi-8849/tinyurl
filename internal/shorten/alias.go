package shorten

import (
	"errors"
	"fmt"

	"github.com/abhi-8849/system-design/tinyurl/internal/idgen"
)

var (
	// ErrInvalidAlias means the custom alias breaks the format or is reserved.
	ErrInvalidAlias = errors.New("shorten: invalid alias")
	// ErrAliasTaken means another link already uses the alias or code.
	ErrAliasTaken = errors.New("shorten: alias already taken")
)

const (
	minAliasLen = 4
	maxAliasLen = 30
)

// reserved holds names that routes use, so an alias can never shadow them.
var reserved = map[string]bool{"api": true, "metrics": true}

// validateAlias enforces 4-30 characters from [A-Za-z0-9_-], not reserved, and not exactly 7 Base62 characters,
// because that is the shape of generated codes and an alias of that shape could clash with one issued later.
func validateAlias(alias string) error {
	if len(alias) < minAliasLen || len(alias) > maxAliasLen {
		return fmt.Errorf("%w: length must be %d to %d", ErrInvalidAlias, minAliasLen, maxAliasLen)
	}
	base62 := true
	for _, c := range alias {
		isAlnum := c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
		if !isAlnum && c != '_' && c != '-' {
			return fmt.Errorf("%w: only letters, digits, '_' and '-' are allowed", ErrInvalidAlias)
		}
		base62 = base62 && isAlnum
	}
	if reserved[alias] {
		return fmt.Errorf("%w: reserved", ErrInvalidAlias)
	}
	if base62 && len(alias) == idgen.CodeLen {
		return fmt.Errorf("%w: %d letters or digits look like a generated code", ErrInvalidAlias, idgen.CodeLen)
	}
	return nil
}
