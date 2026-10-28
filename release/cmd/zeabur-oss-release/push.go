package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"zeabur-oss-release/internal/digests"
	"zeabur-oss-release/internal/oci"
	"zeabur-oss-release/internal/sigstore"
)

// published are the flags of the commands that push: where to record what they pushed.
type published struct {
	digests  string
	subjects string
	noSign   bool
	insecure bool
}

func (p *published) flags(fs *flag.FlagSet) {
	fs.StringVar(&p.digests, "digests", "", "append what was pushed to this digests.txt")
	fs.StringVar(&p.subjects, "subjects", "", "append the subjects to attest to this subjects.json")
	fs.BoolVar(&p.noSign, "no-sign", false, "push without signing (tests)")
	fs.BoolVar(&p.insecure, "insecure", false, "plain HTTP to the registry (tests)")
}

func (p *published) options() oci.Options {
	return oci.Options{Keychain: authn.DefaultKeychain, Insecure: p.insecure}
}

// subject is one entry of subjects.json, what the shared workflow attests.
type subject struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

func (p *published) record(entries []digests.Entry, subjects []subject) error {
	if p.digests != "" {
		if err := digests.Append(p.digests, entries...); err != nil {
			return err
		}
	}
	if p.subjects == "" {
		return nil
	}
	all := []subject{}
	if b, err := os.ReadFile(p.subjects); err == nil {
		if err := json.Unmarshal(b, &all); err != nil {
			return fmt.Errorf("%s: %w", p.subjects, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	b, err := json.Marshal(append(all, subjects...))
	if err != nil {
		return err
	}
	return os.WriteFile(p.subjects, append(b, '\n'), 0o644)
}

func pushImageCommand(ctx context.Context, args []string, tools sigstore.Tools) error {
	fs := flag.NewFlagSet("push-image", flag.ContinueOnError)
	repository := fs.String("repository", "", "the image's repository (ghcr.io/zeabur-oss/zeabur)")
	tag := fs.String("tag", "", "the index's tag, the version without v (0.1.1)")
	label := fs.String("label", "", "the image's name in digests.txt, when the release has several")
	attestPlatforms := fs.Bool("attest-platforms", false, "add each platform image to subjects.json, not only the index")
	var p published
	p.flags(fs)
	if err := fs.Parse(args); err != nil {
		return usageError(err.Error())
	}
	if *repository == "" || *tag == "" || fs.NArg() == 0 {
		return usageError("push-image takes --repository, --tag and <os>/<arch>=<image.tar>...")
	}
	var archives []oci.Archive
	for _, arg := range fs.Args() {
		platform, file, ok := strings.Cut(arg, "=")
		if !ok {
			return usageError(fmt.Sprintf("%q is not <os>/<arch>=<image.tar>", arg))
		}
		pl, err := v1.ParsePlatform(platform)
		if err != nil || pl.OS == "" || pl.Architecture == "" {
			return usageError(fmt.Sprintf("%q is not a platform like linux/amd64", platform))
		}
		archives = append(archives, oci.Archive{Platform: *pl, Path: file})
	}

	pushed, err := oci.PushIndex(ctx, *repository, *tag, archives, p.options())
	if err != nil {
		return err
	}
	var entries []digests.Entry
	var subjects []subject
	for _, image := range pushed {
		if !p.noSign {
			if err := tools.SignImage(ctx, image.Ref.String()); err != nil {
				return err
			}
		}
		kind := "index"
		if image.Platform != nil {
			kind = image.Platform.String()
		}
		entries = append(entries, digests.Entry{Kind: digests.Kind(*label, kind), Ref: image.Ref.String()})
		if image.Platform == nil || *attestPlatforms {
			subjects = append(subjects, subject{Name: image.Ref.Context().Name(), Digest: image.Ref.DigestStr()})
		}
		fmt.Printf("%s  %s\n", digests.Kind(*label, kind), image.Ref)
	}
	return p.record(entries, subjects)
}

func pushChartCommand(ctx context.Context, args []string, tools sigstore.Tools) error {
	fs := flag.NewFlagSet("push-chart", flag.ContinueOnError)
	repository := fs.String("repository", "", "the charts' repository (ghcr.io/zeabur-oss/charts)")
	var p published
	p.flags(fs)
	if err := fs.Parse(args); err != nil {
		return usageError(err.Error())
	}
	if *repository == "" || fs.NArg() != 1 {
		return usageError("push-chart takes --repository and <chart.tgz>")
	}
	chart := fs.Arg(0)
	chartName, version, err := chartMetadata(chart)
	if err != nil {
		return err
	}
	o := p.options()
	ref := strings.TrimPrefix(*repository, "oci://") + "/" + chartName + ":" + version
	tagged, err := name.NewTag(ref, name.StrictValidation)
	if err != nil {
		return err
	}
	if err := oci.Absent(ctx, tagged, o); err != nil {
		return err
	}
	pushArgs := []string{"push", chart, "oci://" + strings.TrimPrefix(*repository, "oci://")}
	if p.insecure {
		pushArgs = append(pushArgs, "--plain-http")
	}
	if err := tools.Run(ctx, "helm", pushArgs...); err != nil {
		return err
	}
	digest, err := oci.Resolve(ctx, ref, o)
	if err != nil {
		return err
	}
	if err := oci.ChartIs(ctx, digest, chart, o); err != nil {
		return err
	}
	if !p.noSign {
		if err := tools.SignImage(ctx, digest.String()); err != nil {
			return err
		}
	}
	fmt.Printf("chart  %s\n", digest)
	return p.record(
		[]digests.Entry{{Kind: "chart", Ref: digest.String()}},
		[]subject{{Name: digest.Context().Name(), Digest: digest.DigestStr()}},
	)
}

var chartField = regexp.MustCompile(`^(name|version):\s*["']?([^"'\s#]+)`)

// chartMetadata reads the name and version from the Chart.yaml in a packaged chart.
func chartMetadata(file string) (string, string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", file, err)
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return "", "", fmt.Errorf("%s has no <chart>/Chart.yaml", file)
		}
		if err != nil {
			return "", "", fmt.Errorf("%s: %w", file, err)
		}
		if strings.Count(h.Name, "/") != 1 || path.Base(h.Name) != "Chart.yaml" {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(tr, 1<<20))
		if err != nil {
			return "", "", err
		}
		fields := map[string]string{}
		for _, line := range strings.Split(string(b), "\n") {
			if m := chartField.FindStringSubmatch(line); m != nil {
				fields[m[1]] = m[2]
			}
		}
		if fields["name"] == "" || fields["version"] == "" {
			return "", "", fmt.Errorf("%s: Chart.yaml has no name or version", file)
		}
		return fields["name"], fields["version"], nil
	}
}
