// Package version holds software release metadata for binaries and RPC.
//
// Inject at link time (semver without requiring a leading "v"):
//
//	go build -ldflags="-X github.com/dewnetwork/dew/version.Version=0.2.0" ./cmd/dew
//
// Protocol freeze tag remains params.PublicTestnetFreezeTag (independent of software version).
package version

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/dewnetwork/dew/params"
)

// Version is the software semver string. Override via -ldflags -X.
// Default "dev" marks local builds that did not inject a release tag.
var Version = "dev"

// SemVer returns the normalized software version (no leading "v"; empty → "dev").
func SemVer() string {
	v := strings.TrimSpace(Version)
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return "dev"
	}
	return v
}

// ClientVersion is the ethereum-compatible web3_clientVersion value:
//
//	Dew/v0.2.0/go1.25.x
func ClientVersion() string {
	return fmt.Sprintf("Dew/v%s/%s", SemVer(), runtime.Version())
}

// Line formats CLI version output, e.g. "dew 0.2.0 (public-testnet-v1)".
func Line(app string) string {
	return fmt.Sprintf("%s %s (%s)", app, SemVer(), params.PublicTestnetFreezeTag)
}
