package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/xxnian95/sateia-cli/internal/cli"
)

var version = "dev"

var releaseVersionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	moduleVersion := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		moduleVersion = info.Main.Version
	}
	command := cli.New(resolvedVersion(version, moduleVersion), os.Stdin, os.Stdout, os.Stderr)
	if err := command.ExecuteContext(ctx); err != nil {
		if writeErr := cli.WriteError(command, os.Args[1:], err, os.Stderr); writeErr != nil {
			fmt.Fprintln(os.Stderr, "Error: write command error:", writeErr)
		}
		os.Exit(1)
	}
}

func resolvedVersion(linkedVersion, moduleVersion string) string {
	if linkedVersion != "" && linkedVersion != "dev" {
		return linkedVersion
	}
	moduleVersion = strings.TrimSpace(moduleVersion)
	if releaseVersionPattern.MatchString(moduleVersion) {
		return moduleVersion
	}
	return "dev"
}
