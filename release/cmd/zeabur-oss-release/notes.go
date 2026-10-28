package main

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"zeabur-oss-release/internal/notes"
)

func newNotesCmd() *cobra.Command {
	var docs string
	cmd := &cobra.Command{
		Use:     "notes <changelog> <version>",
		Short:   "Print a version's CHANGELOG.md section as a release body",
		GroupID: "notes",
		Long: `Print the version's CHANGELOG.md section, without its heading, with the wrapped lines of
paragraphs and list items joined: a release body shows every line break. Fails when the
changelog has no section for the version.

With --docs, the notes end with a link to the product's documentation, which says how to install,
upgrade and verify a release; the body carries no install or verify steps of its own.`,
		Example: `  zeabur-oss-release notes CHANGELOG.md v0.1.1 --docs https://oss.zeabur.com/docs/zeabur/`,
		Args:    cobra.ExactArgs(2),
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
			if docs != "" {
				u, err := url.Parse(docs)
				if err != nil || u.Scheme != "https" || u.Host == "" {
					return fmt.Errorf("--docs %q is not an https URL", docs)
				}
				text += fmt.Sprintf("\nInstall, upgrade and verify: [%s](%s)\n", strings.TrimSuffix(u.Host+u.Path, "/"), docs)
			}
			_, err = io.WriteString(cmd.OutOrStdout(), text)
			return err
		},
	}
	cmd.Flags().StringVar(&docs, "docs", "", "end with a link to this documentation (https://oss.zeabur.com/docs/<product>/)")
	return cmd
}
