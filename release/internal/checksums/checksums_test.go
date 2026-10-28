package checksums

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWriteAndCheck(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "b.tar.gz", "b")
	write(t, dir, "a.tgz", "a")
	write(t, dir, Bundle, "{}") // a bundle from an earlier run is not an asset
	names, err := Write(dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(names, ",") != "a.tgz,b.tar.gz" {
		t.Errorf("listed %v", names)
	}
	got, _ := os.ReadFile(filepath.Join(dir, File))
	want := "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb  a.tgz\n" +
		"3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d  b.tar.gz\n"
	if string(got) != want {
		t.Errorf("SHA256SUMS:\n%s\nwant:\n%s", got, want)
	}
	if err := Check(dir, []string{"b.tar.gz", "a.tgz"}); err != nil {
		t.Error(err)
	}
	if err := Check(dir, nil); err != nil {
		t.Error(err)
	}
	// The format is sha256sum's: it checks the same list.
	if sha256sum, err := exec.LookPath("sha256sum"); err == nil {
		cmd := exec.Command(sha256sum, "--check", "--strict", File)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("sha256sum --check: %v\n%s", err, out)
		}
	}
}

func TestCheckRefuses(t *testing.T) {
	for _, tc := range []struct {
		name     string
		sums     string
		expected []string
		want     string
	}{
		{"changed file", "0000000000000000000000000000000000000000000000000000000000000000  a.tgz\n", nil, "but SHA256SUMS says"},
		{"missing asset", "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb  a.tgz\n", []string{"a.tgz", "b.tar.gz"}, "not the release assets"},
		{"no expectation", "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb  a.tgz\n", []string{}, ""},
		{"path", "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb  ../a.tgz\n", nil, "is not"},
		{"twice", strings.Repeat("ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb  a.tgz\n", 2), nil, "twice"},
		{"empty", "", nil, "no files"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "a.tgz", "a")
			write(t, dir, File, tc.sums)
			err := Check(dir, tc.expected)
			if tc.want == "" {
				if err != nil {
					t.Errorf("err %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err %v, want %q", err, tc.want)
			}
		})
	}
}

func TestWriteRefusesDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(dir); err == nil {
		t.Error("a directory was listed")
	}
	if _, err := Write(t.TempDir()); err == nil {
		t.Error("an empty directory was listed")
	}
}
