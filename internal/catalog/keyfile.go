package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StoredKey returns the key held in the file APIKeyFile names, or "" when the
// provider declares no key file, nothing matches, or the match is empty. See
// StoredKeyPath for which file is read. Surrounding whitespace, including the
// trailing newline a server writes, is removed.
func (p *Provider) StoredKey() string {
	_, key := p.storedKeyFile()
	return key
}

// StoredKeyPath returns the path of the file StoredKey reads: the most recently
// modified regular file matching APIKeyFile, after its env refs (such as
// $HOME) are expanded. It is "" whenever StoredKey is "", so a path is only
// returned for a file that holds a key.
func (p *Provider) StoredKeyPath() string {
	path, _ := p.storedKeyFile()
	return path
}

// storedKeyFile returns the newest file matching APIKeyFile and its trimmed
// contents, or two empty strings when there is no such file or it is empty.
func (p *Provider) storedKeyFile() (path, key string) {
	if p == nil || p.APIKeyFile == "" {
		return "", ""
	}
	matches, err := filepath.Glob(os.ExpandEnv(p.APIKeyFile))
	if err != nil {
		return "", ""
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
		return "", ""
	}
	data, err := os.ReadFile(newest)
	if err != nil {
		return "", ""
	}
	key = strings.TrimSpace(string(data))
	if key == "" {
		return "", ""
	}
	return newest, key
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
