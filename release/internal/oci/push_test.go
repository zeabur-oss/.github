package oci

import (
	"context"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func testRegistry(t *testing.T) (string, Options) {
	t.Helper()
	server := httptest.NewServer(registry.New())
	t.Cleanup(server.Close)
	return strings.TrimPrefix(server.URL, "http://"), Options{Insecure: true}
}

// archive writes a random image for the platform as `docker image save` would.
func archive(t *testing.T, platform v1.Platform) Archive {
	t.Helper()
	img, err := random.Image(256, 2)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	cfg = cfg.DeepCopy()
	cfg.OS, cfg.Architecture = platform.OS, platform.Architecture
	img, err = mutate.ConfigFile(img, cfg)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "image.tar")
	tag, _ := name.NewTag("example/image:release")
	if err := tarball.WriteToFile(path, tag, img); err != nil {
		t.Fatal(err)
	}
	return Archive{Platform: platform, Path: path}
}

var (
	amd64 = v1.Platform{OS: "linux", Architecture: "amd64"}
	arm64 = v1.Platform{OS: "linux", Architecture: "arm64"}
)

func TestPushIndex(t *testing.T) {
	ctx := context.Background()
	host, o := testRegistry(t)
	repo := host + "/zeabur-oss/example"
	archives := []Archive{archive(t, amd64), archive(t, arm64)}

	pushed, err := PushIndex(ctx, repo, "0.1.1", archives, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(pushed) != 3 || pushed[2].Platform != nil {
		t.Fatalf("pushed %v, want two platforms and the index last", pushed)
	}

	// The tag names the index, and the index lists exactly the two images pushed.
	tagged, err := Resolve(ctx, repo+":0.1.1", o)
	if err != nil {
		t.Fatal(err)
	}
	if tagged.String() != pushed[2].Ref.String() {
		t.Errorf("the tag is %s, the index %s", tagged, pushed[2].Ref)
	}
	platforms, err := IndexPlatforms(ctx, pushed[2].Ref, o)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"linux/amd64": pushed[0].Ref.DigestStr(),
		"linux/arm64": pushed[1].Ref.DigestStr(),
	}
	if len(platforms) != 2 || platforms["linux/amd64"] != want["linux/amd64"] || platforms["linux/arm64"] != want["linux/arm64"] {
		t.Errorf("the index lists %v, want %v", platforms, want)
	}

	// No other tag: the platform images were pushed by digest only.
	r, _ := name.NewRepository(repo, name.Insecure)
	tags, err := remote.List(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != "0.1.1" {
		t.Errorf("tags %v, want only 0.1.1", tags)
	}

	// A published version is never replaced.
	_, err = PushIndex(ctx, repo, "0.1.1", archives, o)
	if !errors.Is(err, ErrExists) {
		t.Errorf("second push: %v, want ErrExists", err)
	}
}

func TestPushIndexRefusesBeforeWriting(t *testing.T) {
	ctx := context.Background()
	host, o := testRegistry(t)
	repo := host + "/zeabur-oss/example"
	r, _ := name.NewRepository(repo, name.Insecure)

	for _, tc := range []struct {
		name     string
		archives []Archive
		want     string
	}{
		{"wrong platform", []Archive{archive(t, amd64), {Platform: arm64, Path: archive(t, amd64).Path}}, "not linux/arm64"},
		{"same platform twice", []Archive{archive(t, amd64), archive(t, amd64)}, "two images for linux/amd64"},
		{"missing file", []Archive{{Platform: amd64, Path: filepath.Join(t.TempDir(), "none.tar")}}, "none.tar"},
		{"nothing", nil, "no images"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := PushIndex(ctx, repo, "0.1.1", tc.archives, o)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err %v, want %q", err, tc.want)
			}
			if _, err := remote.List(r); err == nil {
				t.Errorf("the repository exists after a refused push")
			}
		})
	}
}
