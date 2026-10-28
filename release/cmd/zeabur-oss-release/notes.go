package main

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"zeabur-oss-release/internal/notes"
)

func newNotesCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "notes <changelog> <version>",
		Short:   "Print a version's CHANGELOG.md section as a release body",
		GroupID: "notes",
		Long: `Print the version's CHANGELOG.md section, without its heading, with the wrapped lines of
paragraphs and list items joined: a release body shows every line break. Fails when the
changelog has no section for the version.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := os.Open(args[0])
			if err != nil {
				return err
			}
			defer f.Close()
			text, err := notes.Section(f, args[1])
			if err != nil {
				return err
			}
			_, err = io.WriteString(cmd.OutOrStdout(), text)
			return err
		},
	}
}
