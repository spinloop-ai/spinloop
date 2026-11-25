package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StoredKey returns the key held in the file APIKeyFile names, or "" when the
// provider declares no key file, nothing matches, or the match is empty. The
// pattern's env refs (such as $HOME) are expanded before it is globbed; when
// several files match, the most recently modified readable one is used.
// Surrounding whitespace, including the trailing newline a server writes, is
// removed.
func (p *Provider) StoredKey() string {
	if p == nil || p.APIKeyFile == "" {
		return ""
	}
	matches, err := filepath.Glob(os.ExpandEnv(p.APIKeyFile))
	if err != nil {
		return ""
	}
	var newest string
	var newestMod time.Time
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if newest == "" || info.ModTime().After(newestMod) {
			newest, newestMod = m, info.ModTime()
		}
	}
	if newest == "" {
		return ""
	}
	data, err := os.ReadFile(newest)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// KeyResolver returns resolve extended with the provider's stored key: a
// lookup of APIKeyEnv that resolve answers with "" returns StoredKey instead,
// but only when baseURL is a local endpoint, so a key stored by a server on
// this machine is never sent to a remote host. Every other name, and every
// lookup resolve answers, is passed through unchanged. With no key file
// declared, resolve itself is returned.
func (p *Provider) KeyResolver(resolve func(string) string, baseURL string) func(string) string {
	if p == nil || p.APIKeyFile == "" || p.APIKeyEnv == "" || !IsLocalEndpoint(baseURL) {
		return resolve
	}
	return func(name string) string {
		v := resolve(name)
		if v != "" || name != p.APIKeyEnv {
			return v
		}
		return p.StoredKey()
	}
}
