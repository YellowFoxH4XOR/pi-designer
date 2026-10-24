package pirun

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MinVersion is the oldest pi whose RPC mode reports agent_settled.aborted,
// prompt dispositions and --no-mcp, which πDesigner's run lifecycle needs.
const MinVersion = "1.1.0"

// Runtime describes the pi installation πDesigner would use.
type Runtime struct {
	Bin        string `json:"bin"`
	Version    string `json:"version,omitempty"`
	MinVersion string `json:"minVersion"`
	Supported  bool   `json:"supported"`
	Error      string `json:"error,omitempty"`
}

var versionPattern = regexp.MustCompile(`\b(\d+\.\d+\.\d+)\b`)

// Detect resolves pi and reads its version. It never starts an agent session.
func Detect(ctx context.Context, explicit string) Runtime {
	rt := Runtime{MinVersion: MinVersion}
	bin, err := ResolveBin(explicit)
	if err != nil {
		rt.Error = err.Error()
		return rt
	}
	rt.Bin = bin
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "--version").Output()
	if err != nil {
		rt.Error = fmt.Sprintf("pi --version failed: %v", err)
		return rt
	}
	rt.Version = versionPattern.FindString(strings.TrimSpace(string(out)))
	if rt.Version == "" {
		rt.Error = "pi --version did not report a version"
		return rt
	}
	rt.Supported = VersionAtLeast(rt.Version, MinVersion)
	if !rt.Supported {
		rt.Error = fmt.Sprintf("pi %s is too old; πDesigner needs pi %s or newer", rt.Version, MinVersion)
	}
	return rt
}

// VersionAtLeast compares dotted major.minor.patch versions numerically. An
// unparseable version is never treated as supported.
func VersionAtLeast(version, minimum string) bool {
	parse := func(v string) []int {
		parts := strings.SplitN(strings.SplitN(v, "-", 2)[0], ".", 3)
		if len(parts) != 3 {
			return nil
		}
		out := make([]int, 3)
		for i, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil {
				return nil
			}
			out[i] = n
		}
		return out
	}
	have, need := parse(version), parse(minimum)
	if have == nil || need == nil {
		return false
	}
	for i := range have {
		if have[i] != need[i] {
			return have[i] > need[i]
		}
	}
	return true
}
