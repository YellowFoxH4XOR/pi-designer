package omprun

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// sandboxExec is macOS's Seatbelt launcher.
const sandboxExec = "/usr/bin/sandbox-exec"

// Confinement limits what the omp process tree (omp, its tools, shells and
// every command they start) can touch on disk. The kernel enforces it, so it
// holds for bash and scripts as well as omp's own file tools.
//
// Nothing in the user's home directory is readable except Readable and
// Writable paths, and nothing anywhere is writable except Writable paths and
// the temporary/device files a process needs. System locations outside the
// home directory (/usr, /bin, /System, /opt/homebrew, ...) stay readable so
// shells and toolchains work.
type Confinement struct {
	// Writable paths are readable and writable (e.g. the project directory).
	Writable []string
	// Readable paths are read-only (e.g. bundled skills, the extension).
	Readable []string
}

// profile renders the Seatbelt profile. Paths are resolved through symlinks
// because Seatbelt matches resolved paths (/var is /private/var).
func (c Confinement) profile(home string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("directory confinement requires macOS")
	}
	home, err := realDir(home)
	if err != nil {
		return "", err
	}
	var writable, readable []string
	for _, p := range c.Writable {
		r, err := realDir(p)
		if err != nil {
			return "", err
		}
		writable = append(writable, r)
	}
	for _, p := range c.Readable {
		r, err := filepath.EvalSymlinks(p)
		if err != nil {
			continue // nothing to grant for a path that does not exist
		}
		readable = append(readable, r)
	}
	tmp, err := realDir(os.TempDir())
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("(version 1)\n(allow default)\n")
	// Metadata stays visible so path resolution works; contents do not.
	fmt.Fprintf(&b, "(deny file-read-data (subpath %s))\n", sbString(home))
	b.WriteString("(allow file-read-data")
	for _, p := range append(append([]string{}, writable...), readable...) {
		fmt.Fprintf(&b, " (subpath %s)", sbString(p))
	}
	b.WriteString(")\n")
	b.WriteString("(deny file-write* (subpath \"/\"))\n")
	b.WriteString("(allow file-write*")
	for _, p := range append(writable, tmp, "/private/tmp") {
		fmt.Fprintf(&b, " (subpath %s)", sbString(p))
	}
	b.WriteString(" (regex #\"^/dev/\"))\n")
	return b.String(), nil
}

// realDir creates p if needed and returns its resolved absolute path.
func realDir(p string) (string, error) {
	if err := os.MkdirAll(p, 0o755); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// sbString quotes a path as a Seatbelt (Scheme) string literal.
func sbString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// confined wraps the omp invocation in sandbox-exec with opts.Confine plus
// what omp itself needs: its state directory (~/.omp: auth, models, caches),
// its executable, and the runtimes design skills run (node, bun) when they
// are installed inside the home directory.
func confined(bin string, opts Options) (*exec.Cmd, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	c := Confinement{
		Writable: append(append([]string{}, opts.Confine.Writable...), filepath.Join(home, ".omp")),
		Readable: append([]string{}, opts.Confine.Readable...),
	}
	if real, err := filepath.EvalSymlinks(bin); err == nil {
		c.Readable = append(c.Readable, filepath.Dir(real))
	}
	for _, runtimeBin := range []string{"node", "bun"} {
		if p, err := exec.LookPath(runtimeBin); err == nil {
			if real, err := filepath.EvalSymlinks(p); err == nil {
				// <root>/bin/node -> grant <root> (lib, modules), never
				// the whole home directory.
				if root := filepath.Dir(filepath.Dir(real)); strings.HasPrefix(root, home+string(filepath.Separator)) {
					c.Readable = append(c.Readable, root)
				}
			}
		}
	}
	profile, err := c.profile(home)
	if err != nil {
		return nil, err
	}
	return exec.Command(sandboxExec, append([]string{"-p", profile, bin}, opts.args()...)...), nil
}
