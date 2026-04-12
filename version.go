package authmesh

import "fmt"

// Version information for the AuthMesh library
const (
	// Version is the current version of AuthMesh
	Version = "1.0.0"
	
	// GitCommit is the git commit that was compiled. This will be filled in by the compiler.
	GitCommit = "unknown"
	
	// BuildDate is the date when the library was built. This will be filled in by the compiler.
	BuildDate = "unknown"
	
	// GoVersion is the Go version used to compile this binary
	GoVersion = "1.21+"
)

// VersionInfo returns formatted version information
func VersionInfo() string {
	return fmt.Sprintf("AuthMesh v%s (commit: %s, built: %s, go: %s)",
		Version, GitCommit, BuildDate, GoVersion)
}

// APIVersion returns the current API version
func APIVersion() string {
	return "v1"
}
