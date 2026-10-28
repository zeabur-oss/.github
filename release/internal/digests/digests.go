// Package digests reads and writes digests.txt, the release asset that records every image and
// chart a release pushed. Each line is a kind, two spaces and a reference by digest:
//
//	builder linux/amd64  ghcr.io/zeabur-oss/builder@sha256:...
//	builder index  ghcr.io/zeabur-oss/builder@sha256:...
//	chart  ghcr.io/zeabur-oss/charts/zeabur-builder@sha256:...
//
// The kind is an optional label (the image's name, when a release has several), then the platform,
// `index` or `chart`. The file is checksummed and signed with the other assets, so a user can trust
// what it names.
package digests

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var ref = regexp.MustCompile(`^[a-z0-9][a-z0-9._:/-]*@sha256:[0-9a-f]{64}$`)

// Entry is one line.
type Entry struct {
	Kind string // "linux/amd64", "builder index", "chart"
	Ref  string // repository@sha256:...
}

// Kind joins a label and a kind ("builder", "index" → "builder index").
func Kind(label, kind string) string {
	if label == "" {
		return kind
	}
	return label + " " + kind
}

// Append adds entries to the file, creating it if needed.
func Append(path string, entries ...Entry) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !ref.MatchString(e.Ref) {
			f.Close()
			return fmt.Errorf("%q is not a reference by digest", e.Ref)
		}
		if _, err := fmt.Fprintf(f, "%s  %s\n", e.Kind, e.Ref); err != nil {
			f.Close()
			return err
		}
	}
	return f.Close()
}

// Read parses a digests.txt.
func Read(r io.Reader) ([]Entry, error) {
	var entries []Entry
	scanner := bufio.NewScanner(r)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		i := strings.LastIndex(line, " ")
		if i < 0 {
			return nil, fmt.Errorf("digests.txt line %d has no kind", n)
		}
		e := Entry{Kind: strings.Join(strings.Fields(line[:i]), " "), Ref: line[i+1:]}
		if e.Kind == "" || !ref.MatchString(e.Ref) {
			return nil, fmt.Errorf("digests.txt line %d is not \"<kind>  <repository>@sha256:<digest>\"", n)
		}
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}

// One returns the reference of the only entry of the kind.
func One(entries []Entry, kind string) (string, error) {
	found := ""
	for _, e := range entries {
		if e.Kind != kind {
			continue
		}
		if found != "" {
			return "", fmt.Errorf("digests.txt has more than one %s", kind)
		}
		found = e.Ref
	}
	if found == "" {
		return "", fmt.Errorf("digests.txt has no %s", kind)
	}
	return found, nil
}
