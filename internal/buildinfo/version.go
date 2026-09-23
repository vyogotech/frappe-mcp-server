// Package buildinfo reports the build's own version, so no binary carries a version pinned in its source.
package buildinfo

import "runtime/debug"

// Version is the module version a released binary was built from, or "(devel)" from a working tree.
func Version() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(devel)"
}
