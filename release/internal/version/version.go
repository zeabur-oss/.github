// Package version reads a release tag: a SemVer 2.0.0 version with a v in front, such as v0.1.2
// or v0.1.2-beta.1. A version with a pre-release part is a pre-release; build metadata (+...) is
// refused, since an OCI tag cannot carry a +.
package version

import (
	"fmt"
	"regexp"
)

// The regular expression of https://semver.org/spec/v2.0.0.html, without build metadata.
var tag = regexp.MustCompile(`^v((0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*))` +
	`(-(0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(\.(0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*)?$`)

// Version is a release tag.
type Version struct {
	Tag        string // v0.1.2-beta.1
	Plain      string // 0.1.2-beta.1, as manifests, images and charts carry it
	Normal     string // 0.1.2, whose CHANGELOG.md section a pre-release shares
	Prerelease bool
}

// Parse reads a release tag.
func Parse(s string) (Version, error) {
	m := tag.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("%q is not a version like v1.2.3 or v1.2.3-beta.1", s)
	}
	return Version{Tag: s, Plain: s[1:], Normal: m[1], Prerelease: m[5] != ""}, nil
}
