// Package oci pushes a release's container images and reads them back.
//
// Each platform image is pushed by its digest alone, and only the index that lists them gets the
// version tag. A registry keeps a manifest pushed by digest as long as something refers to it, and
// the index does; so there is no temporary tag to learn a digest by, and nothing to clean up.
package oci

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// Archive is one platform's image, as `docker image save` writes it.
type Archive struct {
	Platform v1.Platform
	Path     string
}

// Pushed is an image as pushed: the repository and the digest that names it there.
type Pushed struct {
	Platform *v1.Platform // nil for the index
	Ref      name.Digest
}

// Options are the registry options: authentication and, in tests, the transport.
type Options struct {
	Keychain  authn.Keychain
	Transport http.RoundTripper
	Insecure  bool // plain HTTP, for a registry on localhost in tests
}

func (o Options) remote(ctx context.Context) []remote.Option {
	opts := []remote.Option{remote.WithContext(ctx)}
	if o.Keychain != nil {
		opts = append(opts, remote.WithAuthFromKeychain(o.Keychain))
	}
	if o.Transport != nil {
		opts = append(opts, remote.WithTransport(o.Transport))
	}
	return opts
}

func (o Options) nameOptions() []name.Option {
	opts := []name.Option{name.StrictValidation}
	if o.Insecure {
		opts = append(opts, name.Insecure)
	}
	return opts
}

// ErrExists is returned when the tag is already in the registry: a published version is never
// replaced.
var ErrExists = errors.New("already exists")

// PushIndex pushes each archive by digest, then an index of exactly those images tagged tag. It
// refuses before writing anything when the tag exists, or when an archive's image is not for the
// platform given with it. The index is the last thing pushed: if anything fails before it, the tag
// does not exist and the run can be repeated.
func PushIndex(ctx context.Context, repository, tag string, archives []Archive, o Options) ([]Pushed, error) {
	repo, err := name.NewRepository(repository, o.nameOptions()...)
	if err != nil {
		return nil, err
	}
	tagged := repo.Tag(tag)
	if err := Absent(ctx, tagged, o); err != nil {
		return nil, err
	}
	if len(archives) == 0 {
		return nil, errors.New("no images to push")
	}

	images := make([]v1.Image, len(archives))
	seen := map[string]bool{}
	for i, a := range archives {
		if seen[a.Platform.String()] {
			return nil, fmt.Errorf("two images for %s", a.Platform.String())
		}
		seen[a.Platform.String()] = true
		img, err := tarball.ImageFromPath(a.Path, nil)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", a.Path, err)
		}
		cfg, err := img.ConfigFile()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", a.Path, err)
		}
		got := v1.Platform{OS: cfg.OS, Architecture: cfg.Architecture, Variant: cfg.Variant}
		if !got.Equals(a.Platform) {
			return nil, fmt.Errorf("%s is an image for %s, not %s", a.Path, got.String(), a.Platform.String())
		}
		images[i] = img
	}

	var pushed []Pushed
	index := mutate.IndexMediaType(empty.Index, types.DockerManifestList)
	for i, img := range images {
		digest, err := img.Digest()
		if err != nil {
			return nil, err
		}
		ref := repo.Digest(digest.String())
		if err := remote.Write(ref, img, o.remote(ctx)...); err != nil {
			return nil, fmt.Errorf("push %s: %w", ref, err)
		}
		platform := archives[i].Platform
		pushed = append(pushed, Pushed{Platform: &platform, Ref: ref})
		index = mutate.AppendManifests(index, mutate.IndexAddendum{
			Add:        img,
			Descriptor: v1.Descriptor{Platform: &platform},
		})
	}

	digest, err := index.Digest()
	if err != nil {
		return nil, err
	}
	if err := remote.WriteIndex(tagged, index, o.remote(ctx)...); err != nil {
		return nil, fmt.Errorf("push %s: %w", tagged, err)
	}
	return append(pushed, Pushed{Ref: repo.Digest(digest.String())}), nil
}

// Absent returns nil when the reference is not in the registry, ErrExists when it is, and the
// registry's error when it cannot tell.
func Absent(ctx context.Context, ref name.Reference, o Options) error {
	_, err := remote.Head(ref, o.remote(ctx)...)
	if err == nil {
		return fmt.Errorf("%s %w; inspect it before trying again", ref, ErrExists)
	}
	var terr *transport.Error
	if errors.As(err, &terr) && terr.StatusCode == http.StatusNotFound {
		return nil
	}
	return fmt.Errorf("check %s: %w", ref, err)
}

// Resolve returns the digest a tag names now.
func Resolve(ctx context.Context, ref string, o Options) (name.Digest, error) {
	r, err := name.ParseReference(ref, o.nameOptions()...)
	if err != nil {
		return name.Digest{}, err
	}
	desc, err := remote.Head(r, o.remote(ctx)...)
	if err != nil {
		return name.Digest{}, err
	}
	return r.Context().Digest(desc.Digest.String()), nil
}

// IndexPlatforms returns the platforms an index lists, each with its image's digest.
func IndexPlatforms(ctx context.Context, ref name.Digest, o Options) (map[string]string, error) {
	idx, err := remote.Index(ref, o.remote(ctx)...)
	if err != nil {
		return nil, err
	}
	manifest, err := idx.IndexManifest()
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, m := range manifest.Manifests {
		if m.Platform == nil {
			return nil, fmt.Errorf("%s lists %s without a platform", ref, m.Digest)
		}
		p := m.Platform.String()
		if _, dup := out[p]; dup {
			return nil, fmt.Errorf("%s lists %s twice", ref, p)
		}
		out[p] = m.Digest.String()
	}
	return out, nil
}

// ChartMediaType is the media type of a Helm chart's content layer.
const ChartMediaType = "application/vnd.cncf.helm.chart.content.v1.tar+gzip"

// ChartIs checks that the chart at ref is the packaged file: its content layer has the file's
// digest. A chart pushed by another run under the same version would not.
func ChartIs(ctx context.Context, ref name.Digest, file string, o Options) error {
	img, err := remote.Image(ref, o.remote(ctx)...)
	if err != nil {
		return err
	}
	manifest, err := img.Manifest()
	if err != nil {
		return err
	}
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	want, _, err := v1.SHA256(f)
	if err != nil {
		return err
	}
	for _, layer := range manifest.Layers {
		if layer.MediaType == ChartMediaType {
			if layer.Digest != want {
				return fmt.Errorf("%s holds the chart %s, not %s (%s)", ref, layer.Digest, file, want)
			}
			return nil
		}
	}
	return fmt.Errorf("%s has no chart content layer", ref)
}
