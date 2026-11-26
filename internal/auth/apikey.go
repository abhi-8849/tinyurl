package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrUnauthenticated means the request has no API key, or one that is not known.
var ErrUnauthenticated = errors.New("auth: unauthenticated")

// header carries the API key on create and delete.
const header = "X-API-Key"

// APIKeys maps API keys to owners. It is read-only after construction, so it is safe for concurrent use.
type APIKeys struct {
	owners map[string]string
}

// NewAPIKeys parses "key:owner,key:owner". Keys and owners must be non-empty, and a key may appear only once.
func NewAPIKeys(spec string) (*APIKeys, error) {
	owners := make(map[string]string)
	for _, pair := range strings.Split(spec, ",") {
		if strings.TrimSpace(pair) == "" {
			continue
		}
		key, owner, ok := strings.Cut(pair, ":")
		key, owner = strings.TrimSpace(key), strings.TrimSpace(owner)
		if !ok || key == "" || owner == "" {
			return nil, errors.New("auth: each API key entry must look like key:owner")
		}
		if _, dup := owners[key]; dup {
			return nil, fmt.Errorf("auth: duplicate API key for owner %q", owner)
		}
		owners[key] = owner
	}
	if len(owners) == 0 {
		return nil, errors.New("auth: no API keys configured")
	}
	return &APIKeys{owners: owners}, nil
}

// Owner returns the owner of the request's API key, or ErrUnauthenticated. The key is never included in an error.
func (a *APIKeys) Owner(r *http.Request) (string, error) {
	owner, ok := a.owners[r.Header.Get(header)]
	if !ok {
		return "", ErrUnauthenticated
	}
	return owner, nil
}
