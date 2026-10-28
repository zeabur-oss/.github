package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"zeabur-oss-release/internal/checksums"
	"zeabur-oss-release/internal/digests"
	"zeabur-oss-release/internal/oci"
	"zeabur-oss-release/internal/sigstore"
)

func newChecksumsCmd(tools sigstore.Tools) *cobra.Command {
	var noSign bool
	cmd := &cobra.Command{
		Use:     "checksums <dir>",
		Short:   "Write SHA256SUMS over every file in a directory and sign it",
		GroupID: "publish",
		Long: `Write <dir>/SHA256SUMS, in sha256sum's format, over every file in <dir>, and sign it into
<dir>/SHA256SUMS.sigstore.json. Prints the files listed.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			names, err := checksums.Write(dir)
			if err != nil {
				return err
			}
			for _, n := range names {
				fmt.Fprintln(cmd.OutOrStdout(), n)
			}
			if noSign {
				return nil
			}
			return tools.SignBlob(cmd.Context(), filepath.Join(dir, checksums.File), filepath.Join(dir, checksums.Bundle))
		},
	}
	cmd.Flags().BoolVar(&noSign, "no-sign", false, "write SHA256SUMS without signing it (tests)")
	return cmd
}

const signerHelp = `
The signer must be zeabur-oss/.github's release.yml, run for --repository at refs/tags/<version>.
RELEASE_CERT_IDENTITY_REGEXP, RELEASE_SIGNER_WORKFLOW, RELEASE_SOURCE_REPOSITORY and
RELEASE_SOURCE_REF override it, as the shared release workflow sets them.`

// release are the flags every verify command takes: whose release it is.
type release struct {
	repository string
	version    string
}

func (r *release) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&r.repository, "repository", "", "the repository released (zeabur-oss/zeabur)")
	cmd.Flags().StringVar(&r.version, "version", "", "the version released (v0.1.1)")
	_ = cmd.MarkFlagRequired("version")
}

func (r *release) identity() (sigstore.Identity, error) {
	return sigstore.IdentityFromEnv(r.repository, r.version)
}

// readDigests reads the release's digests.txt.
func readDigests(file string) ([]digests.Entry, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return digests.Read(f)
}

func addInsecureFlag(fs *pflag.FlagSet, insecure *bool) {
	fs.BoolVar(insecure, "insecure", false, "plain HTTP to the registry (tests)")
}

// readOnly reads a registry as a user does, with no login: a publisher's login must not hide a
// private package.
func readOnly(insecure bool) oci.Options {
	return oci.Options{Keychain: authn.NewMultiKeychain(), Insecure: insecure}
}

func newVerifyAssetsCmd(tools sigstore.Tools) *cobra.Command {
	var r release
	cmd := &cobra.Command{
		Use:     "verify-assets --version <vX.Y.Z> <dir> <asset>...",
		Short:   "Check the signed SHA256SUMS covers exactly the assets, and each asset's provenance",
		GroupID: "verify",
		Long: `Check that <dir>/SHA256SUMS is signed by the release workflow, lists exactly the assets
named, and that they match it; then each asset's build provenance.` + "\n" + signerHelp,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := r.identity()
			if err != nil {
				return err
			}
			ctx, out := cmd.Context(), cmd.OutOrStdout()
			dir, assets := args[0], args[1:]
			if err := tools.VerifyBlob(ctx, id, filepath.Join(dir, checksums.File), filepath.Join(dir, checksums.Bundle)); err != nil {
				return err
			}
			if err := checksums.Check(dir, assets); err != nil {
				return err
			}
			fmt.Fprintln(out, "checksums verified:", strings.Join(slices.Sorted(slices.Values(assets)), " "))
			for _, asset := range assets {
				if err := tools.VerifyProvenance(ctx, id, filepath.Join(dir, asset)); err != nil {
					return err
				}
				fmt.Fprintln(out, "provenance verified:", asset)
			}
			return nil
		},
	}
	r.addFlags(cmd)
	return cmd
}

func newVerifyImageCmd(tools sigstore.Tools) *cobra.Command {
	var (
		r           release
		file, label string
		insecure    bool
	)
	cmd := &cobra.Command{
		Use:     "verify-image --version <vX.Y.Z> --digests <digests.txt> <os>/<arch>...",
		Short:   "Check the recorded index holds exactly the recorded platform images, and their signatures",
		GroupID: "verify",
		Long: `Check that the index digests.txt records lists exactly the platform images it records for the
platforms named, that every one of them is signed, and the index's build provenance.` + "\n" + signerHelp,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := r.identity()
			if err != nil {
				return err
			}
			entries, err := readDigests(file)
			if err != nil {
				return err
			}
			index, err := digests.One(entries, digests.Kind(label, "index"))
			if err != nil {
				return err
			}
			want := map[string]string{}
			refs := []string{index}
			for _, platform := range args {
				ref, err := digests.One(entries, digests.Kind(label, platform))
				if err != nil {
					return err
				}
				if repo(ref) != repo(index) {
					return fmt.Errorf("%s is in %s, the index in %s", platform, repo(ref), repo(index))
				}
				want[platform] = ref[strings.LastIndex(ref, "@")+1:]
				refs = append(refs, ref)
			}

			ctx, out := cmd.Context(), cmd.OutOrStdout()
			indexRef, err := name.NewDigest(index, name.StrictValidation)
			if err != nil {
				return err
			}
			got, err := oci.IndexPlatforms(ctx, indexRef, readOnly(insecure))
			if err != nil {
				return err
			}
			if !maps.Equal(got, want) {
				return fmt.Errorf("the index %s holds %v, not the recorded %v", index, got, want)
			}
			fmt.Fprintln(out, "index verified:", index)
			for _, ref := range refs {
				if err := tools.VerifyImage(ctx, id, ref); err != nil {
					return err
				}
				fmt.Fprintln(out, "signature verified:", ref)
			}
			if err := tools.VerifyProvenance(ctx, id, "oci://"+index); err != nil {
				return err
			}
			fmt.Fprintln(out, "provenance verified:", index)
			return nil
		},
	}
	r.addFlags(cmd)
	cmd.Flags().StringVar(&file, "digests", "", "the release's digests.txt")
	cmd.Flags().StringVar(&label, "label", "", "the image's label in digests.txt")
	addInsecureFlag(cmd.Flags(), &insecure)
	_ = cmd.MarkFlagRequired("digests")
	return cmd
}

func newVerifyChartCmd(tools sigstore.Tools) *cobra.Command {
	var (
		r        release
		file     string
		insecure bool
	)
	cmd := &cobra.Command{
		Use:     "verify-chart --version <vX.Y.Z> --digests <digests.txt> <chart.tgz>",
		Short:   "Check the recorded chart is the release's asset, and its signature",
		GroupID: "verify",
		Long: `Check that the chart digests.txt records is, in the registry, the packaged chart given (the
release's asset), that it is signed, and its build provenance.` + "\n" + signerHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := r.identity()
			if err != nil {
				return err
			}
			entries, err := readDigests(file)
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
			ctx, out := cmd.Context(), cmd.OutOrStdout()
			if err := oci.ChartIs(ctx, ref, args[0], readOnly(insecure)); err != nil {
				return err
			}
			fmt.Fprintln(out, "chart in the registry is the asset:", chart)
			if err := tools.VerifyImage(ctx, id, chart); err != nil {
				return err
			}
			if err := tools.VerifyProvenance(ctx, id, "oci://"+chart); err != nil {
				return err
			}
			fmt.Fprintln(out, "signature and provenance verified:", chart)
			return nil
		},
	}
	r.addFlags(cmd)
	cmd.Flags().StringVar(&file, "digests", "", "the release's digests.txt")
	addInsecureFlag(cmd.Flags(), &insecure)
	_ = cmd.MarkFlagRequired("digests")
	return cmd
}

func repo(ref string) string { return ref[:strings.LastIndex(ref, "@")] }
