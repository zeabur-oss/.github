package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

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

func (p *published) addFlags(fs *pflag.FlagSet) {
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

func newPushImageCmd(tools sigstore.Tools) *cobra.Command {
	var (
		repository, tag, label string
		attestPlatforms        bool
		p                      published
	)
	cmd := &cobra.Command{
		Use:     "push-image --repository <repository> --tag <X.Y.Z> <os>/<arch>=<image.tar>...",
		Short:   "Push each platform image by digest, then their index under the version tag",
		GroupID: "publish",
		Long: `Push each platform image (as docker image save wrote it) by its digest alone, then an index
of exactly those images tagged X.Y.Z, and sign every digest. Refuses before pushing anything when
the tag exists, or when an image is not for the platform given with it.`,
		Example: `  zeabur-oss-release push-image --repository ghcr.io/zeabur-oss/zeabur --tag 0.1.1 \
    --digests published/digests.txt --subjects published/subjects.json \
    linux/amd64=amd64/image.tar linux/arm64=arm64/image.tar`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var archives []oci.Archive
			for _, arg := range args {
				platform, file, ok := strings.Cut(arg, "=")
				if !ok {
					return fmt.Errorf("%q is not <os>/<arch>=<image.tar>", arg)
				}
				pl, err := v1.ParsePlatform(platform)
				if err != nil || pl.OS == "" || pl.Architecture == "" {
					return fmt.Errorf("%q is not a platform like linux/amd64", platform)
				}
				archives = append(archives, oci.Archive{Platform: *pl, Path: file})
			}

			ctx := cmd.Context()
			pushed, err := oci.PushIndex(ctx, repository, tag, archives, p.options())
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
				entries = append(entries, digests.Entry{Kind: digests.Kind(label, kind), Ref: image.Ref.String()})
				if image.Platform == nil || attestPlatforms {
					subjects = append(subjects, subject{Name: image.Ref.Context().Name(), Digest: image.Ref.DigestStr()})
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", digests.Kind(label, kind), image.Ref)
			}
			return p.record(entries, subjects)
		},
	}
	f := cmd.Flags()
	f.StringVar(&repository, "repository", "", "the image's repository (ghcr.io/zeabur-oss/zeabur)")
	f.StringVar(&tag, "tag", "", "the index's tag, the version without v (0.1.1)")
	f.StringVar(&label, "label", "", "the image's name in digests.txt, when the release has several")
	f.BoolVar(&attestPlatforms, "attest-platforms", false, "add each platform image to subjects.json, not only the index")
	p.addFlags(f)
	_ = cmd.MarkFlagRequired("repository")
	_ = cmd.MarkFlagRequired("tag")
	return cmd
}

func newPushChartCmd(tools sigstore.Tools) *cobra.Command {
	var (
		repository string
		p          published
	)
	cmd := &cobra.Command{
		Use:     "push-chart --repository <repository> <chart.tgz>",
		Short:   "Push a packaged chart, check the registry holds this file, and sign it",
		GroupID: "publish",
		Example: `  zeabur-oss-release push-chart --repository ghcr.io/zeabur-oss/charts \
    --digests published/digests.txt --subjects published/subjects.json zeabur-0.1.1.tgz`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			chart := args[0]
			chartName, version, err := chartMetadata(chart)
			if err != nil {
				return err
			}
			o := p.options()
			repo := strings.TrimPrefix(repository, "oci://")
			ref := repo + "/" + chartName + ":" + version
			tagged, err := name.NewTag(ref, name.StrictValidation)
			if err != nil {
				return err
			}
			if err := oci.Absent(ctx, tagged, o); err != nil {
				return err
			}
			pushArgs := []string{"push", chart, "oci://" + repo}
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
			fmt.Fprintf(cmd.OutOrStdout(), "chart  %s\n", digest)
			return p.record(
				[]digests.Entry{{Kind: "chart", Ref: digest.String()}},
				[]subject{{Name: digest.Context().Name(), Digest: digest.DigestStr()}},
			)
		},
	}
	cmd.Flags().StringVar(&repository, "repository", "", "the charts' repository (ghcr.io/zeabur-oss/charts)")
	p.addFlags(cmd.Flags())
	_ = cmd.MarkFlagRequired("repository")
	return cmd
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
