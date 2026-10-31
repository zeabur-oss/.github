package notes

import (
	"errors"
	"strings"
	"testing"
)

const changelog = `# Changelog

## [Unreleased]

- Not this.

## [0.1.1] - 2026-10-28

The first published release. 0.1.0 was never published: zeabur-oss withdrew it in every
repository, so everything listed under 0.1.0 in
[CHANGELOG.md](https://example.com/CHANGELOG.md) ships first in
0.1.1.

### Changed

- Releases are built, signed and attested by the shared release workflow of
  [zeabur-oss/.github](https://github.com/zeabur-oss/.github). Every signature names
  ` + "`release.yml@…`" + ` as its signer.
- A release checks the catalog
  and Zeabur Identity.
  - A nested item
    continued.

` + "```sh" + `
cosign verify \
  --certificate-identity-regexp x
## not a heading
` + "```" + `

| a | b |
|---|---|
| 1 | 2 |

Two spaces end this line` + "  " + `
and this one starts a new line.
> A quote
> on two lines.

## [0.1.0] - 2026-10-28 [YANKED]

Never published.
`

func TestSection(t *testing.T) {
	got, err := Section(strings.NewReader(changelog), "v0.1.1")
	if err != nil {
		t.Fatal(err)
	}
	want := `The first published release. 0.1.0 was never published: zeabur-oss withdrew it in every repository, so everything listed under 0.1.0 in [CHANGELOG.md](https://example.com/CHANGELOG.md) ships first in 0.1.1.

### Changed

- Releases are built, signed and attested by the shared release workflow of [zeabur-oss/.github](https://github.com/zeabur-oss/.github). Every signature names ` + "`release.yml@…`" + ` as its signer.
- A release checks the catalog and Zeabur Identity.
  - A nested item continued.

` + "```sh" + `
cosign verify \
  --certificate-identity-regexp x
## not a heading
` + "```" + `

| a | b |
|---|---|
| 1 | 2 |

Two spaces end this line` + "  " + `
and this one starts a new line.
> A quote
> on two lines.
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSectionVersions(t *testing.T) {
	for _, v := range []string{"0.1.0", "v0.1.0", "v0.1.0-beta.1", "0.1.0-rc.2"} {
		got, err := Section(strings.NewReader(changelog), v)
		if err != nil || got != "Never published.\n" {
			t.Errorf("%s: got %q, %v", v, got, err)
		}
	}
}

func TestSectionKnopeHeading(t *testing.T) {
	text := "## 0.1.2 (2026-10-31)\n\n### Fixes\n\n- One.\n\n## 0.1.1 (2026-10-28)\n\nOlder.\n"
	got, err := Section(strings.NewReader(text), "v0.1.2-rc.1")
	if err != nil || got != "### Fixes\n\n- One.\n" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestSectionPrerelease(t *testing.T) {
	text := "## 0.1.2-rc.1 (2026-10-31)\n\n- Second.\n\n## 0.1.2-rc.0 (2026-10-30)\n\n- First.\n"
	for v, want := range map[string]string{"v0.1.2-rc.1": "- Second.\n", "v0.1.2-rc.0": "- First.\n"} {
		got, err := Section(strings.NewReader(text), v)
		if err != nil || got != want {
			t.Errorf("%s: got %q, %v", v, got, err)
		}
	}
	// The release does not read its pre-releases' sections.
	if _, err := Section(strings.NewReader(text), "v0.1.2"); err == nil {
		t.Error("v0.1.2 read a pre-release's section")
	}
}

func TestSectionMissing(t *testing.T) {
	for _, v := range []string{"v0.2.0", "v0.1", "0.1.1.1", "v0.1.1+build"} {
		_, err := Section(strings.NewReader(changelog), v)
		if err == nil {
			t.Errorf("%s: no error", v)
		}
	}
	// A heading for a longer version is not this version's: 0.1.10 is not 0.1.1.
	var missing ErrNoSection
	for _, text := range []string{"## [0.1.10]\n\ntext\n", "## 0.1.10 (2026-10-31)\n\ntext\n"} {
		_, err := Section(strings.NewReader(text), "v0.1.1")
		if !errors.As(err, &missing) {
			t.Errorf("0.1.10 for 0.1.1: %v", err)
		}
	}
	// A heading with nothing under it is no section.
	_, err := Section(strings.NewReader("## [0.1.1]\n\n## [0.1.0]\n\ntext\n"), "v0.1.1")
	if !errors.As(err, &missing) {
		t.Errorf("empty section: %v", err)
	}
}

func TestUnwrapLeavesBlocks(t *testing.T) {
	in := []string{
		"text",
		"<details>",
		"text",
		"---",
		"1. one",
		"2) two",
		"    indented",
		"[ref]: https://example.com",
		"tail",
	}
	got := strings.Join(Unwrap(in), "\n")
	want := strings.Join([]string{
		"text",
		"<details>",
		"text",
		"---",
		"1. one",
		"2) two indented",
		"[ref]: https://example.com",
		"tail",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
