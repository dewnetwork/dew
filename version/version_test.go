package version

import (
	"runtime"
	"strings"
	"testing"

	"github.com/dewnetwork/dew/params"
)

func TestSemVerDefault(t *testing.T) {
	// Package default is "dev" for un-injected local builds.
	if got := SemVer(); got != "dev" {
		t.Fatalf("default SemVer() = %q, want dev", got)
	}
}

func TestSemVerNormalizes(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	cases := []struct {
		in, want string
	}{
		{"0.2.0", "0.2.0"},
		{"v0.2.0", "0.2.0"},
		{"  v1.0.0-rc.1  ", "1.0.0-rc.1"},
		{"", "dev"},
		{"   ", "dev"},
	}
	for _, tc := range cases {
		Version = tc.in
		if got := SemVer(); got != tc.want {
			t.Errorf("Version=%q SemVer()=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestClientVersion(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	Version = "0.2.0"
	got := ClientVersion()
	wantPrefix := "Dew/v0.2.0/"
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("ClientVersion() = %q, want prefix %q", got, wantPrefix)
	}
	if !strings.HasSuffix(got, runtime.Version()) {
		t.Fatalf("ClientVersion() = %q, want suffix %q", got, runtime.Version())
	}
}

func TestLine(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	Version = "v0.2.1"
	got := Line("dew")
	want := "dew 0.2.1 (" + params.PublicTestnetFreezeTag + ")"
	if got != want {
		t.Fatalf("Line(dew) = %q, want %q", got, want)
	}
}
