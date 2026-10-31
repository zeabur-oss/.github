package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"zeabur-oss-release/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "version <vVERSION>",
		Short:   "Check a release tag and print what the release reads from it",
		GroupID: "notes",
		Long: `Check that the tag is a SemVer 2.0.0 version with a v in front and no build metadata, and print
it as key=value lines, for $GITHUB_OUTPUT or a script:

  plain       the version without v, as manifests, images and charts carry it
  normal      the version without its pre-release part, whose CHANGELOG.md section a pre-release
              without its own uses
  prerelease  true for a version with a pre-release part (-beta.1, -rc.1), else false`,
		Example: `  zeabur-oss-release version v0.1.2-beta.1`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := version.Parse(args[0])
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "plain=%s\nnormal=%s\nprerelease=%t\n", v.Plain, v.Normal, v.Prerelease)
			return err
		},
	}
}
