package redact

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// tokenSalt is generated once per process. In a future phase we can plumb a per-bundle salt.
var tokenSalt []byte

func init() {
	tokenSalt = make([]byte, 32)
	_, _ = rand.Read(tokenSalt)
}

// tokenizeValue produces a stable token for a given sensitive value.
// typeHint is used as a visible prefix (e.g., SECRET, APIKEY). It is uppercased and sanitized.
func tokenizeValue(value []byte, typeHint string) string {
	if len(typeHint) == 0 {
		typeHint = "SECRET"
	}
	typeHint = sanitizeTypeHint(typeHint)

	mac := hmac.New(sha256.New, tokenSalt)
	mac.Write(value)
	sum := mac.Sum(nil)

	// Use a short hex to keep tokens readable while unique enough for correlation.
	// 12 hex chars ~ 48 bits, sufficient for collision resistance in this context.
	short := hex.EncodeToString(sum)[:12]
	return "***TOKEN_" + typeHint + "_" + strings.ToUpper(short) + "***"
}

func sanitizeTypeHint(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "SECRET"
	}
	// Keep alnum and underscore only
	b := strings.Builder{}
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' {
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return "SECRET"
	}
	return out
}
