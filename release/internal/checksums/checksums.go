// Package checksums writes and checks SHA256SUMS, in the format of `sha256sum`, so that users can
// check a release with `sha256sum --check` and nothing else.
package checksums

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// File is the name of the checksum list; Bundle is its Sigstore bundle.
const (
	File   = "SHA256SUMS"
	Bundle = "SHA256SUMS.sigstore.json"
)

// Write lists every regular file in dir except the list and its bundle, sorted by name, and
// returns the names listed. A directory or link in dir is refused: a release's assets are files.
func Write(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	var b strings.Builder
	for _, e := range entries {
		if e.Name() == File || e.Name() == Bundle {
			continue
		}
		if !e.Type().IsRegular() {
			return nil, fmt.Errorf("%s is not a regular file", filepath.Join(dir, e.Name()))
		}
		sum, err := hashFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&b, "%s  %s\n", sum, e.Name())
		names = append(names, e.Name())
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("%s has no files to list", dir)
	}
	return names, os.WriteFile(filepath.Join(dir, File), []byte(b.String()), 0o644)
}

var line = regexp.MustCompile(`^([0-9a-f]{64}) [ *]([^/\\]+)$`)

// Check reads dir's SHA256SUMS, requires it to list exactly the expected names, and checks every
// file's digest. An empty expected list accepts whatever the list names.
func Check(dir string, expected []string) error {
	f, err := os.Open(filepath.Join(dir, File))
	if err != nil {
		return err
	}
	defer f.Close()
	sums := map[string]string{}
	var listed []string
	scanner := bufio.NewScanner(f)
	for n := 1; scanner.Scan(); n++ {
		m := line.FindStringSubmatch(scanner.Text())
		if m == nil {
			return fmt.Errorf("%s line %d is not \"<sha256>  <file>\"", File, n)
		}
		if _, dup := sums[m[2]]; dup {
			return fmt.Errorf("%s lists %s twice", File, m[2])
		}
		sums[m[2]] = m[1]
		listed = append(listed, m[2])
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(listed) == 0 {
		return fmt.Errorf("%s lists no files", File)
	}
	if len(expected) > 0 {
		want := slices.Sorted(slices.Values(expected))
		got := slices.Sorted(slices.Values(listed))
		if !slices.Equal(want, got) {
			return fmt.Errorf("%s lists %v, not the release assets %v", File, got, want)
		}
	}
	for _, name := range listed {
		sum, err := hashFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if sum != sums[name] {
			return fmt.Errorf("%s: sha256 %s, but %s says %s", name, sum, File, sums[name])
		}
	}
	return nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
