package shorten

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrInvalidURL means the long URL is not an acceptable http or https address.
var ErrInvalidURL = errors.New("shorten: invalid url")

// maxURLLen caps stored URLs so one request cannot make a row arbitrarily large.
const maxURLLen = 2048

// validateURL accepts only absolute http/https URLs within the length cap that do not point back at this service,
// since a link to ourselves could redirect in a loop.
func validateURL(raw, ownHost string) error {
	if len(raw) > maxURLLen {
		return fmt.Errorf("%w: longer than %d bytes", ErrInvalidURL, maxURLLen)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: scheme must be http or https", ErrInvalidURL)
	}
	if u.Hostname() == "" {
		return fmt.Errorf("%w: missing host", ErrInvalidURL)
	}
	if strings.EqualFold(u.Host, ownHost) {
		return fmt.Errorf("%w: points at this service", ErrInvalidURL)
	}
	return nil
}
