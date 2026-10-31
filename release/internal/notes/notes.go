// Package notes takes a version's section out of CHANGELOG.md and turns it into the body of a
// GitHub release.
//
// CHANGELOG.md is wrapped at a fixed width, as Markdown allows: a line break inside a paragraph is
// a soft break, rendered as a space. A release body is not rendered that way; GitHub shows every
// line break in it, so a wrapped paragraph comes out as ragged short lines. Section joins the
// lines of each paragraph and list item and leaves everything whose line breaks mean something
// (code, tables, headings, HTML, hard breaks) as it is.
package notes

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"zeabur-oss-release/internal/version"
)

var (
	fence     = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
	listItem  = regexp.MustCompile(`^ *([-*+]|\d{1,9}[.)])( +|$)`)
	heading   = regexp.MustCompile(`^ {0,3}#{1,6}( |$)`)
	thematic  = regexp.MustCompile(`^ {0,3}([-*_])( *[-*_]){2,} *$`)
	reference = regexp.MustCompile(`^ {0,3}\[[^\]]+\]: `)
)

// ErrNoSection is returned when the changelog has no section for the version, or one with no text.
type ErrNoSection struct{ Version string }

func (e ErrNoSection) Error() string {
	return fmt.Sprintf("CHANGELOG.md has no section ## %s with text", e.Version)
}

// Section returns the text of the section headed `## X.Y.Z`, as knope writes it, or `## [X.Y.Z]`,
// as Keep a Changelog does, optionally followed by other text after a space (` (2026-10-31)`,
// ` - 2026-10-31`). The heading is left out and soft line breaks are joined. The version may be
// given with or without its v. A pre-release reads its own section, as knope writes one for each
// (## 0.1.2-rc.1), or else the section of its normal version (## 0.1.2).
func Section(changelog io.Reader, v string) (string, error) {
	parsed, err := version.Parse("v" + strings.TrimPrefix(v, "v"))
	if err != nil {
		return "", err
	}
	data, err := io.ReadAll(changelog)
	if err != nil {
		return "", err
	}
	text, err := section(string(data), parsed.Plain)
	var missing ErrNoSection
	if parsed.Prerelease && errors.As(err, &missing) {
		return section(string(data), parsed.Normal)
	}
	return text, err
}

func section(changelog, plain string) (string, error) {
	headings := []string{"## " + plain, "## [" + plain + "]"}

	var lines []string
	found := false
	inFence := ""
	scanner := bufio.NewScanner(strings.NewReader(changelog))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if !found {
			for _, h := range headings {
				if line == h || strings.HasPrefix(line, h+" ") {
					found = true
				}
			}
			continue
		}
		// A `## ` line inside a code block is code, not the next section.
		if m := fence.FindStringSubmatch(line); m != nil {
			switch {
			case inFence == "":
				inFence = m[1][:3]
			case strings.HasPrefix(strings.TrimLeft(line, " "), inFence):
				inFence = ""
			}
		} else if inFence == "" && (strings.HasPrefix(line, "## ") || line == "##") {
			break
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	text := strings.TrimSpace(strings.Join(Unwrap(lines), "\n"))
	if !found || text == "" {
		return "", ErrNoSection{Version: plain}
	}
	return text + "\n", nil
}

// Unwrap joins the soft line breaks of paragraphs and list items. A line continues the one before
// it when both are text: not blank, not inside a code block, and the later one does not start a
// block of its own (a list item, heading, quote, table row, HTML, fence or thematic break). A
// previous line ending in two spaces or a backslash is a hard break and stays.
func Unwrap(lines []string) []string {
	var out []string
	inFence := ""
	joinable := false // the last line of out is text that a continuation may join
	for _, line := range lines {
		if m := fence.FindStringSubmatch(line); m != nil {
			switch {
			case inFence == "":
				inFence = m[1][:3]
			case strings.HasPrefix(strings.TrimLeft(line, " "), inFence):
				inFence = ""
			}
			out = append(out, line)
			joinable = false
			continue
		}
		if inFence != "" {
			out = append(out, line)
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out = append(out, "")
			joinable = false
			continue
		}
		if joinable && !startsBlock(line) {
			prev := out[len(out)-1]
			if !strings.HasSuffix(prev, "  ") && !strings.HasSuffix(prev, `\`) {
				out[len(out)-1] = strings.TrimRight(prev, " ") + " " + trimmed
				continue
			}
		}
		out = append(out, line)
		joinable = isText(line)
	}
	return out
}

// startsBlock reports whether the line starts a block rather than continuing a paragraph.
func startsBlock(line string) bool {
	t := strings.TrimLeft(line, " ")
	return listItem.MatchString(line) || heading.MatchString(line) || thematic.MatchString(line) ||
		reference.MatchString(line) ||
		strings.HasPrefix(t, ">") || strings.HasPrefix(t, "|") || strings.HasPrefix(t, "<")
}

// isText reports whether a line may be continued by the next: paragraph or list item text, not a
// heading, table row, HTML, reference definition or thematic break. Quote lines are continued only
// by further quote lines, which startsBlock keeps apart.
func isText(line string) bool {
	t := strings.TrimLeft(line, " ")
	if len(line)-len(t) >= 4 && !listItem.MatchString(line) {
		// Indented four or more: a list item's continuation, or indented code. Joining code would
		// break it, and a continuation reads the same unjoined; keep it.
		return false
	}
	return !(heading.MatchString(line) || thematic.MatchString(line) || reference.MatchString(line) ||
		strings.HasPrefix(t, ">") || strings.HasPrefix(t, "|") || strings.HasPrefix(t, "<"))
}
