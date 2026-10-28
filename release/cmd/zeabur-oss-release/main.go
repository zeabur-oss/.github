// Command zeabur-oss-release holds the steps of a zeabur-oss release that every repository does
// the same way, so that each repository's scripts/release.sh calls them instead of carrying its
// own copy. See README.md in this directory.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"zeabur-oss-release/internal/sigstore"
)

const usage = `usage: zeabur-oss-release <command> [arguments]

Publishing (in the release workflow, with its registry login and identity token):
  push-image --repository R --tag X.Y.Z [--label L] <os>/<arch>=<image.tar>...
      push each platform image by digest, then their index tagged X.Y.Z; sign every digest
  push-chart --repository R <chart.tgz>
      push a packaged chart, check the registry holds this file, sign it
      both take --digests <digests.txt> and --subjects <subjects.json> to record what they pushed;
      push-image --attest-platforms also lists the platform images in subjects.json
  checksums <dir>
      write <dir>/SHA256SUMS over every file in <dir> and sign it (SHA256SUMS.sigstore.json)

Verifying (anyone, no login; gh must be signed in to read attestations):
  verify-assets --repository O/N --version vX.Y.Z <dir> <asset>...
      SHA256SUMS is signed by the release workflow, lists exactly these assets, and they match;
      each asset's build provenance
  verify-image --repository O/N --version vX.Y.Z --digests <digests.txt> [--label L] <os>/<arch>...
      the recorded index holds exactly the recorded platform images; every signature; provenance
  verify-chart --repository O/N --version vX.Y.Z --digests <digests.txt> <chart.tgz>
      the recorded chart is the asset; its signature and provenance
      The signer defaults to zeabur-oss/.github's release.yml run for O/N at refs/tags/vX.Y.Z;
      RELEASE_CERT_IDENTITY_REGEXP, RELEASE_SIGNER_WORKFLOW, RELEASE_SOURCE_REPOSITORY and
      RELEASE_SOURCE_REF override it.

Notes:
  notes <changelog> <version>
      print the version's CHANGELOG.md section with its wrapped lines joined, as a release body
      shows every line break; exit 1 if there is none
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	tools := sigstore.Tools{Run: sigstore.Exec}
	var err error
	switch command, args := os.Args[1], os.Args[2:]; command {
	case "notes":
		err = notesCommand(args, os.Stdout)
	case "push-image":
		err = pushImageCommand(ctx, args, tools)
	case "push-chart":
		err = pushChartCommand(ctx, args, tools)
	case "checksums":
		err = checksumsCommand(ctx, args, tools)
	case "verify-assets":
		err = verifyAssetsCommand(ctx, args, tools)
	case "verify-image":
		err = verifyImageCommand(ctx, args, tools)
	case "verify-chart":
		err = verifyChartCommand(ctx, args, tools)
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		err = usageError(fmt.Sprintf("unknown command %q", command))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "zeabur-oss-release:", err)
		var u usageError
		if errors.As(err, &u) {
			fmt.Fprint(os.Stderr, usage)
			os.Exit(2)
		}
		os.Exit(1)
	}
}

// usageError is a mistake in the command line, reported with the usage and exit code 2.
type usageError string

func (e usageError) Error() string { return string(e) }
