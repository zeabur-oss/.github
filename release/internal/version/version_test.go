package version

import "testing"

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Version
	}{
		{"v0.1.2", Version{"v0.1.2", "0.1.2", "0.1.2", false}},
		{"v0.1.2-beta.1", Version{"v0.1.2-beta.1", "0.1.2-beta.1", "0.1.2", true}},
		{"v1.0.0-rc.1", Version{"v1.0.0-rc.1", "1.0.0-rc.1", "1.0.0", true}},
		{"v1.0.0-alpha", Version{"v1.0.0-alpha", "1.0.0-alpha", "1.0.0", true}},
		{"v1.0.0-0.3.7", Version{"v1.0.0-0.3.7", "1.0.0-0.3.7", "1.0.0", true}},
		{"v1.0.0-x-y.0a", Version{"v1.0.0-x-y.0a", "1.0.0-x-y.0a", "1.0.0", true}},
	} {
		got, err := Parse(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", tc.in, got, err, tc.want)
		}
	}
}

func TestParseRefuses(t *testing.T) {
	for _, in := range []string{
		"0.1.2", "v0.1", "v01.0.0", "v1.0.0-01", "v1.0.0-", "v1.0.0-beta..1",
		"v1.0.0+build", "v1.0.0-beta+build", "v1.0.0-beta_1", "latest",
	} {
		if v, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = %+v, want an error", in, v)
		}
	}
}
