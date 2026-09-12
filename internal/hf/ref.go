// Package hf reads Hugging Face model repos and turns one into the provider
// selection a Spinloop states. It reads the Hub's two JSON endpoints and the
// local Hugging Face and llama.cpp caches; it never downloads weights —
// fetching those stays the engine's job.
//
// The package is a leaf: stdlib only, plus internal/contextsize for the
// lenient window sizes a reference may override with. Environment access and
// the cache roots reach it as arguments, so a test points them at a temp
// tree and nothing here touches the machine's real caches.
package hf

import (
	"fmt"
	"net/url"
	"strings"
)

// Ref is a Hugging Face model reference in one of the forms a user pastes:
// a bare org/model, a hf.co/ or huggingface.co/ prefixed form, or a full
// https:// URL with a /tree/<revision> or /blob/<revision>/<file> path — any
// of them with an @revision suffix and/or a llama.cpp-style :QUANT suffix,
// written org/model@revision:QUANT.
type Ref struct {
	Owner    string
	Name     string
	Revision string
	// Quant is the :QUANT suffix, when one was named.
	Quant string
	// File is the /blob/<revision>/<file> path, when one was named.
	File string
}

// Repo is the identity of a model repo, without its revision.
type Repo struct {
	Owner string
	Name  string
}

// Repo is Ref's owner and name, for the lookups that need no revision.
func (r Ref) Repo() Repo { return Repo{Owner: r.Owner, Name: r.Name} }

// defaultRevision is the revision a reference names when it names none.
const defaultRevision = "main"

// exampleReference is what the failure for an unparseable reference shows, so
// the repair is one paste away.
const exampleReference = "unsloth/Qwen3.6-35B-A3B-GGUF"

// cleanPart reports whether s is a plain repo part: non-empty, with none of
// the characters the reference syntax itself uses.
func cleanPart(s string) bool {
	if s == "" {
		return false
	}
	return !strings.ContainsAny(s, "@:/ \t")
}

// cutSuffix strips a trailing "<sep><token>" from s, reporting the token. The
// token must be non-empty and carry none of the reference syntax's own
// characters, so a scheme's colon in http:// never reads as a :QUANT.
func cutSuffix(s string, sep byte) (string, bool) {
	i := strings.LastIndexByte(s, sep)
	if i < 0 {
		return "", false
	}
	token := s[i+1:]
	if token == "" || strings.ContainsAny(token, string(rune(sep))+"/@ \t") {
		return "", false
	}
	return token, true
}

// ParseRef parses a pasted reference into its parts, failing with a message
// that names what was wrong and shows the form a reference takes.
func ParseRef(s string) (Ref, error) {
	original := strings.TrimSpace(s)
	if original == "" {
		return Ref{}, fmt.Errorf(
			"%q is not a Hugging Face model reference: expected org/model, e.g. %s", original, exampleReference)
	}

	// The suffixes are written @revision first, :QUANT second, so they are
	// stripped in the reverse order. A URL may carry them appended to its
	// path the same way.
	rest := original
	quant := ""
	if q, ok := cutSuffix(rest, ':'); ok {
		quant = q
		rest = rest[:len(rest)-len(q)-1]
	}
	suffixRev := ""
	if r, ok := cutSuffix(rest, '@'); ok {
		suffixRev = r
		rest = rest[:len(rest)-len(r)-1]
	}

	if strings.Contains(rest, "://") {
		repo, pathRev, file, err := parseURLForm(rest, original)
		if err != nil {
			return Ref{}, err
		}
		ref := Ref{Owner: repo.Owner, Name: repo.Name, File: file, Quant: quant}
		switch {
		case suffixRev != "" && pathRev != "" && suffixRev != pathRev:
			return Ref{}, fmt.Errorf(
				"%q names two different revisions: @%s and the URL's %s — name one", original, suffixRev, pathRev)
		case pathRev != "":
			ref.Revision = pathRev
		case suffixRev != "":
			ref.Revision = suffixRev
		default:
			ref.Revision = defaultRevision
		}
		if ref.File != "" && ref.Quant != "" {
			return Ref{}, fmt.Errorf(
				"%q names both a file and a :QUANT suffix: name the one that picks the model", original)
		}
		return ref, nil
	}

	// The bare forms: org/model, or the same behind a Hugging Face host.
	parts := strings.Split(rest, "/")
	if len(parts) == 3 && (parts[0] == "hf.co" || parts[0] == "huggingface.co") {
		parts = parts[1:]
	}
	// A host with nothing but an organisation behind it names no model.
	if len(parts) == 2 && (parts[0] == "hf.co" || parts[0] == "huggingface.co") {
		return Ref{}, fmt.Errorf(
			"%q is not a Hugging Face model reference: expected org/model, e.g. %s", original, exampleReference)
	}
	if len(parts) != 2 || !cleanPart(parts[0]) || !cleanPart(parts[1]) {
		return Ref{}, fmt.Errorf(
			"%q is not a Hugging Face model reference: expected org/model, e.g. %s", original, exampleReference)
	}
	ref := Ref{Owner: parts[0], Name: parts[1], Quant: quant, Revision: defaultRevision}
	if suffixRev != "" {
		ref.Revision = suffixRev
	}
	return ref, nil
}

// parseURLForm reads a pasted Hugging Face URL: the repo from its path, and
// the revision and file from a /tree/<revision> or /blob/<revision>/<file>
// tail when one is present. original is the reference as pasted, for the
// error messages.
func parseURLForm(rest, original string) (Repo, string, string, error) {
	u, err := url.Parse(rest)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return Repo{}, "", "", fmt.Errorf(
			"%q is not a Hugging Face URL: expected https://huggingface.co/org/model", original)
	}
	if u.Hostname() != "huggingface.co" && u.Hostname() != "hf.co" {
		return Repo{}, "", "", fmt.Errorf(
			"%q is not a Hugging Face URL: the host is %s, expected huggingface.co or hf.co", original, u.Hostname())
	}

	seg := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch len(seg) {
	case 2:
		if !cleanPart(seg[0]) || !cleanPart(seg[1]) {
			return Repo{}, "", "", fmt.Errorf(
				"%q is not a Hugging Face model reference: the URL's path must be /org/model, e.g. /%s", original, exampleReference)
		}
		return Repo{Owner: seg[0], Name: seg[1]}, "", "", nil
	case 4:
		if seg[2] != "tree" || !cleanPart(seg[0]) || !cleanPart(seg[1]) || !cleanPart(seg[3]) {
			return Repo{}, "", "", fmt.Errorf(
				"%q is not a Hugging Face model reference: the URL's path is not /org/model/tree/<revision>", original)
		}
		return Repo{Owner: seg[0], Name: seg[1]}, seg[3], "", nil
	case 5:
		if seg[2] != "blob" || !cleanPart(seg[0]) || !cleanPart(seg[1]) || !cleanPart(seg[3]) || seg[4] == "" {
			return Repo{}, "", "", fmt.Errorf(
				"%q is not a Hugging Face model reference: the URL's path is not /org/model/blob/<revision>/<file>", original)
		}
		return Repo{Owner: seg[0], Name: seg[1]}, seg[3], seg[4], nil
	default:
		return Repo{}, "", "", fmt.Errorf(
			"%q is not a Hugging Face model reference: the URL's path must be /org/model, /org/model/tree/<revision> or /org/model/blob/<revision>/<file>", original)
	}
}
