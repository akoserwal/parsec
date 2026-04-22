package version

// Version is the application version. Override at build time via:
//
//	go build -ldflags "-X github.com/project-kessel/parsec/internal/version.Version=x.y.z"
var Version = "dev"
