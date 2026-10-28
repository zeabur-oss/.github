package main

import (
	"context"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"

	"zeabur-oss-release/internal/checksums"
	"zeabur-oss-release/internal/digests"
	"zeabur-oss-release/internal/oci"
	"zeabur-oss-release/internal/sigstore"
)

func checksumsCommand(ctx context.Context, args []string, tools sigstore.Tools) error {
	fs := flag.NewFlagSet("checksums", flag.ContinueOnError)
	noSign := fs.Bool("no-sign", false, "write SHA256SUMS without signing it (tests)")
	if err := fs.Parse(args); err != nil {
		return usageError(err.Error())
	}
	if fs.NArg() != 1 {
		return usageError("checksums takes <assets directory>")
	}
	dir := fs.Arg(0)
	names, err := checksums.Write(dir)
	if err != nil {
		return err
	}
	for _, n := range names {
		fmt.Println(n)
	}
	if *noSign {
		return nil
	}
	return tools.SignBlob(ctx, filepath.Join(dir, checksums.File), filepath.Join(dir, checksums.Bundle))
}

// verifyFlags are the flags every verify command takes: whose release it is.
type verifyFlags struct {
	repository string
	version    string
}

func (v *verifyFlags) flags(fs *flag.FlagSet) {
	fs.StringVar(&v.repository, "repository", "", "the repository released (zeabur-oss/zeabur); RELEASE_SOURCE_REPOSITORY overrides it")
	fs.StringVar(&v.version, "version", "", "the version released (v0.1.1)")
}

func (v *verifyFlags) identity() (sigstore.Identity, error) {
	if v.version == "" {
		return sigstore.Identity{}, usageError("--version is required")
	}
	return sigstore.IdentityFromEnv(v.repository, v.version)
}

func verifyAssetsCommand(ctx context.Context, args []string, tools sigstore.Tools) error {
	fs := flag.NewFlagSet("verify-assets", flag.ContinueOnError)
	var v verifyFlags
	v.flags(fs)
	if err := fs.Parse(args); err != nil {
		return usageError(err.Error())
	}
	if fs.NArg() < 2 {
		return usageError("verify-assets takes <directory> <asset>...")
	}
	id, err := v.identity()
	if err != nil {
		return err
	}
	dir, assets := fs.Arg(0), fs.Args()[1:]
	// The list is the release workflow's, for this repository at this tag; then it covers exactly
	// the assets, and they match it.
	if err := tools.VerifyBlob(ctx, id, filepath.Join(dir, checksums.File), filepath.Join(dir, checksums.Bundle)); err != nil {
		return err
	}
	if err := checksums.Check(dir, assets); err != nil {
		return err
	}
	fmt.Println("checksums verified:", strings.Join(slices.Sorted(slices.Values(assets)), " "))
	for _, asset := range assets {
		if err := tools.VerifyProvenance(ctx, id, filepath.Join(dir, asset)); err != nil {
			return err
		}
		fmt.Println("provenance verified:", asset)
	}
	return nil
}

func verifyImageCommand(ctx context.Context, args []string, tools sigstore.Tools) error {
	fs := flag.NewFlagSet("verify-image", flag.ContinueOnError)
	var v verifyFlags
	v.flags(fs)
	file := fs.String("digests", "", "the release's digests.txt")
	label := fs.String("label", "", "the image's label in digests.txt")
	insecure := fs.Bool("insecure", false, "plain HTTP to the registry (tests)")
	if err := fs.Parse(args); err != nil {
		return usageError(err.Error())
	}
	if *file == "" || fs.NArg() == 0 {
		return usageError("verify-image takes --digests and the platforms the index must hold (linux/amd64 ...)")
	}
	id, err := v.identity()
	if err != nil {
		return err
	}
	f, err := os.Open(*file)
	if err != nil {
		return err
	}
	entries, err := digests.Read(f)
	f.Close()
	if err != nil {
		return err
	}
	index, err := digests.One(entries, digests.Kind(*label, "index"))
	if err != nil {
		return err
	}
	want := map[string]string{}
	refs := []string{index}
	for _, platform := range fs.Args() {
		ref, err := digests.One(entries, digests.Kind(*label, platform))
		if err != nil {
			return err
		}
		if repo(ref) != repo(index) {
			return fmt.Errorf("%s is in %s, the index in %s", platform, repo(ref), repo(index))
		}
		want[platform] = ref[strings.LastIndex(ref, "@")+1:]
		refs = append(refs, ref)
	}

	// Anonymous, as a user pulls: a publisher's login must not hide a private package.
	o := oci.Options{Keychain: authn.NewMultiKeychain(), Insecure: *insecure}
	indexRef, err := name.NewDigest(index, name.StrictValidation)
	if err != nil {
		return err
	}
	got, err := oci.IndexPlatforms(ctx, indexRef, o)
	if err != nil {
		return err
	}
	if !maps.Equal(got, want) {
		return fmt.Errorf("the index %s holds %v, not the recorded %v", index, got, want)
	}
	fmt.Println("index verified:", index)
	for _, ref := range refs {
		if err := tools.VerifyImage(ctx, id, ref); err != nil {
			return err
		}
		fmt.Println("signature verified:", ref)
	}
	if err := tools.VerifyProvenance(ctx, id, "oci://"+index); err != nil {
		return err
	}
	fmt.Println("provenance verified:", index)
	return nil
}

func verifyChartCommand(ctx context.Context, args []string, tools sigstore.Tools) error {
	fs := flag.NewFlagSet("verify-chart", flag.ContinueOnError)
	var v verifyFlags
	v.flags(fs)
	file := fs.String("digests", "", "the release's digests.txt")
	insecure := fs.Bool("insecure", false, "plain HTTP to the registry (tests)")
	if err := fs.Parse(args); err != nil {
		return usageError(err.Error())
	}
	if *file == "" || fs.NArg() != 1 {
		return usageError("verify-chart takes --digests and <chart.tgz>, the release's asset")
	}
	id, err := v.identity()
	if err != nil {
		return err
	}
	f, err := os.Open(*file)
	if err != nil {
		return err
	}
	entries, err := digests.Read(f)
	f.Close()
	if err != nil {
		return err
	}
	chart, err := digests.One(entries, "chart")
	if err != nil {
		return err
	}
	ref, err := name.NewDigest(chart, name.StrictValidation)
	if err != nil {
		return err
	}
	o := oci.Options{Keychain: authn.NewMultiKeychain(), Insecure: *insecure}
	if err := oci.ChartIs(ctx, ref, fs.Arg(0), o); err != nil {
		return err
	}
	fmt.Println("chart in the registry is the asset:", chart)
	if err := tools.VerifyImage(ctx, id, chart); err != nil {
		return err
	}
	if err := tools.VerifyProvenance(ctx, id, "oci://"+chart); err != nil {
		return err
	}
	fmt.Println("signature and provenance verified:", chart)
	return nil
}

func repo(ref string) string { return ref[:strings.LastIndex(ref, "@")] }
