package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/kurnhyalcantara/temtem/config"
)

// Build metadata, injected via -ldflags at build time (see the Makefile). They
// default to "dev"/"none" for `go run` and unstamped builds.
var (
	buildVersion = "dev"
	buildCommit  = "none"
	buildDate    = "unknown"
)

// configPath is bound to the persistent --config flag and consumed by the
// subcommands that load configuration.
var configPath string

// newRootCmd assembles the command tree. The root has no run behaviour of its
// own; `serve` runs the service.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "temtem",
		Short:         "temtem service CLI",
		Long:          "temtem is a gRPC-first microservice. The serve command runs the gRPC server, its REST gateway, and the ops server.",
		Version:       buildVersion,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&configPath, "config", config.DefaultPath,
		"path to config yaml (\"\" to use defaults + env only)")

	root.AddCommand(newServeCmd(), newVersionCmd())
	return root
}

// Execute runs the CLI and converts a command error into a non-zero exit.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		slog.Error("command failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
