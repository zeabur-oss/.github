package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"zeabur-oss-release/internal/digests"
	"zeabur-oss-release/internal/sigstore"
)

// run runs the command line with the tools, and returns its output.
func run(tools sigstore.Tools, args ...string) (string, error) {
	var out strings.Builder
	root := newRootCmd(tools)
	root.SetArgs(args)
	root.SetOut(&out)
	root.SetErr(io.Discard)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

// recorder stands in for cosign, gh and helm, and records what was run.
type recorder struct{ calls []string }

func (r *recorder) tools() sigstore.Tools {
	return sigstore.Tools{Run: func(_ context.Context, name string, args ...string) error {
		r.calls = append(r.calls, name+" "+strings.Join(args, " "))
		return nil
	}}
}

func imageTar(t *testing.T, arch string) string {
	t.Helper()
	img, err := random.Image(128, 1)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := img.ConfigFile()
	cfg = cfg.DeepCopy()
	cfg.OS, cfg.Architecture = "linux", arch
	if img, err = mutate.ConfigFile(img, cfg); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), arch+".tar")
	tag, _ := name.NewTag("example:" + arch)
	if err := tarball.WriteToFile(path, tag, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPushImageRecordsAndSigns(t *testing.T) {
	server := httptest.NewServer(registry.New())
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")
	out := t.TempDir()
	digestsFile, subjectsFile := filepath.Join(out, "digests.txt"), filepath.Join(out, "subjects.json")
	// subjects.json may hold an earlier command's subjects already.
	if err := os.WriteFile(subjectsFile, []byte(`[{"name":"earlier","digest":"sha256:00"}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	var r recorder
	_, err := run(r.tools(), "push-image",
		"--repository", host+"/zeabur-oss/builder", "--tag", "0.1.1", "--label", "builder",
		"--digests", digestsFile, "--subjects", subjectsFile, "--insecure",
		"linux/amd64="+imageTar(t, "amd64"), "linux/arm64="+imageTar(t, "arm64"),
	)
	if err != nil {
		t.Fatal(err)
	}

	f, _ := os.Open(digestsFile)
	entries, err := digests.Read(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, e := range entries {
		kinds = append(kinds, e.Kind)
	}
	if strings.Join(kinds, ",") != "builder linux/amd64,builder linux/arm64,builder index" {
		t.Errorf("digests.txt kinds %v", kinds)
	}
	index, _ := digests.One(entries, "builder index")

	// Every digest is signed, by digest, never by tag.
	if len(r.calls) != 3 {
		t.Fatalf("calls %v", r.calls)
	}
	for i, e := range entries {
		if r.calls[i] != "cosign sign --yes "+e.Ref {
			t.Errorf("call %d is %q, want a signature of %s", i, r.calls[i], e.Ref)
		}
	}

	// Only the index is attested, after what was there.
	var subjects []subject
	b, _ := os.ReadFile(subjectsFile)
	if err := json.Unmarshal(b, &subjects); err != nil {
		t.Fatal(err)
	}
	if len(subjects) != 2 || subjects[0].Name != "earlier" ||
		subjects[1].Name+"@"+subjects[1].Digest != index {
		t.Errorf("subjects %v, want earlier and %s", subjects, index)
	}
}

func TestPushImageUsage(t *testing.T) {
	var r recorder
	for _, args := range [][]string{
		{"--tag", "0.1.1", "linux/amd64=a.tar"},
		{"--repository", "ghcr.io/x/y", "--tag", "0.1.1"},
		{"--repository", "ghcr.io/x/y", "--tag", "0.1.1", "a.tar"},
		{"--repository", "ghcr.io/x/y", "--tag", "0.1.1", "amd64=a.tar"},
	} {
		if _, err := run(r.tools(), append([]string{"push-image"}, args...)...); err == nil {
			t.Errorf("%v: no error", args)
		}
	}
	if len(r.calls) != 0 {
		t.Errorf("ran %v", r.calls)
	}
}

func TestVerifyImageRefusesMismatch(t *testing.T) {
	server := httptest.NewServer(registry.New())
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")
	dir := t.TempDir()
	file := filepath.Join(dir, "digests.txt")
	var r recorder
	if _, err := run(r.tools(), "push-image",
		"--repository", host+"/o/zeabur", "--tag", "0.1.1", "--digests", file, "--insecure", "--no-sign",
		"linux/amd64="+imageTar(t, "amd64"), "linux/arm64="+imageTar(t, "arm64"),
	); err != nil {
		t.Fatal(err)
	}
	verify := func(platforms ...string) error {
		_, err := run(r.tools(), append([]string{"verify-image",
			"--repository", "o/zeabur", "--version", "v0.1.1", "--digests", file, "--insecure",
		}, platforms...)...)
		return err
	}
	if err := verify("linux/amd64", "linux/arm64"); err != nil {
		t.Fatal(err)
	}
	wantCalls := 4 // the index and two images signed, the index's provenance
	if len(r.calls) != wantCalls || !strings.HasPrefix(r.calls[3], "gh attestation verify oci://") {
		t.Errorf("calls %v", r.calls)
	}
	// The index holds a platform the release does not record.
	if err := verify("linux/amd64"); err == nil || !strings.Contains(err.Error(), "not the recorded") {
		t.Errorf("one platform: %v", err)
	}
	// digests.txt records a platform image the index does not hold.
	other := v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("0", 64)}
	if err := digests.Append(file, digests.Entry{Kind: "linux/riscv64", Ref: host + "/o/zeabur@" + other.String()}); err != nil {
		t.Fatal(err)
	}
	if err := verify("linux/amd64", "linux/arm64", "linux/riscv64"); err == nil {
		t.Error("a platform outside the index was accepted")
	}
}

func TestNotesCommand(t *testing.T) {
	file := filepath.Join(t.TempDir(), "CHANGELOG.md")
	if err := os.WriteFile(file, []byte("## [0.1.1]\n\nOne\nparagraph.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := run(sigstore.Tools{}, "notes", file, "v0.1.1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "One paragraph.\n" {
		t.Errorf("notes %q", got)
	}
	got, err = run(sigstore.Tools{}, "notes", file, "v0.1.1", "--docs", "https://oss.zeabur.com/docs/zeabur/")
	if err != nil {
		t.Fatal(err)
	}
	want := "One paragraph.\n\nInstall, upgrade and verify: [oss.zeabur.com/docs/zeabur](https://oss.zeabur.com/docs/zeabur/)\n"
	if got != want {
		t.Errorf("notes --docs %q, want %q", got, want)
	}
	if _, err := run(sigstore.Tools{}, "notes", file, "v0.1.1", "--docs", "oss.zeabur.com/docs"); err == nil {
		t.Error("no error for a URL without https")
	}
	if _, err := run(sigstore.Tools{}, "notes", file, "v0.2.0"); err == nil {
		t.Error("no error for a missing section")
	}
}

func TestVersionCommand(t *testing.T) {
	got, err := run(sigstore.Tools{}, "version", "v0.1.2-beta.1")
	if err != nil {
		t.Fatal(err)
	}
	if want := "plain=0.1.2-beta.1\nnormal=0.1.2\nprerelease=true\n"; got != want {
		t.Errorf("version %q, want %q", got, want)
	}
	got, err = run(sigstore.Tools{}, "version", "v0.1.2")
	if err != nil {
		t.Fatal(err)
	}
	if want := "plain=0.1.2\nnormal=0.1.2\nprerelease=false\n"; got != want {
		t.Errorf("version %q, want %q", got, want)
	}
	if _, err := run(sigstore.Tools{}, "version", "0.1.2"); err == nil {
		t.Error("no error for a version without v")
	}
}
