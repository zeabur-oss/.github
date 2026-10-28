// Command zeabur-oss-release holds the steps of a zeabur-oss release that every repository does
// the same way, so that each repository's scripts/release.sh calls them instead of carrying its
// own copy. See README.md in the release directory.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"zeabur-oss-release/internal/sigstore"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := newRootCmd(sigstore.Tools{Run: sigstore.Exec}).ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

func newRootCmd(tools sigstore.Tools) *cobra.Command {
	root := &cobra.Command{
		Use:   "zeabur-oss-release",
		Short: "Publish, sign and verify a zeabur-oss release",
		Long: `The steps of a release that every zeabur-oss repository does the same way. A repository's
scripts/release.sh keeps what is its own and calls these commands for the rest.`,
		SilenceUsage: true,
	}
	root.AddGroup(
		&cobra.Group{ID: "publish", Title: "Publishing (in the release workflow, with its registry login and identity token):"},
		&cobra.Group{ID: "verify", Title: "Verifying (anyone, no registry login; gh must be signed in to read attestations):"},
		&cobra.Group{ID: "notes", Title: "Release notes:"},
	)
	// Run from scripts and workflows, not typed: no completion command.
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(
		newPushImageCmd(tools),
		newPushChartCmd(tools),
		newChecksumsCmd(tools),
		newVerifyAssetsCmd(tools),
		newVerifyImageCmd(tools),
		newVerifyChartCmd(tools),
		newNotesCmd(),
	)
	return root
}
