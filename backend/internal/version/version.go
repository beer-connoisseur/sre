// Package version holds the build version. It is injected at build time:
//
//	go build -ldflags "-X urlshort/internal/version.version=1.0.0"
package version

var version = "dev"

func Get() string {
	return version
}
