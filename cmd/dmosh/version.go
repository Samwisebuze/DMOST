package main

import (
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is the build's version. It is overridden at link time:
//
//	go build -ldflags "-X main.version=$(git describe --tags)" .
//
// and otherwise falls back to whatever the toolchain stamped into the binary.
var version = ""

// newVersionCommand reports the build. It touches no daemon, which makes it the
// one command that still answers when nothing else is running.
func (m *Main) newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the dmosh version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println(buildVersion())
			return nil
		},
	}
}

// buildVersion resolves the version to report: the linker's value if there is
// one, else the VCS revision the go tool stamped in, else "devel".
func buildVersion() string {
	if version != "" {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "devel"
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && s.Value != "" {
			return s.Value
		}
	}
	return "devel"
}
