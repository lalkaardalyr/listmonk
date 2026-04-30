package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
	pflag "github.com/spf13/pflag"
)

const (
	appName    = "listmonk"
	appVersion = "v3.0.0"
)

// App holds the global application state.
type App struct {
	log    *log.Logger
	ko     *koanf.Koanf
	fs     stuffbin.FileSystem
}

var (
	// Global logger instance.
	// Using log.Ldate|log.Ltime for human-readable timestamps in logs.
	// Keeping log.Lshortfile for cleaner, more readable log output.
	logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile)
)

func main() {
	// Define and parse CLI flags.
	flags := pflag.NewFlagSet("config", pflag.ContinueOnError)
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", appName)
		flags.PrintDefaults()
		os.Exit(0)
	}

	flags.StringSlice("config", []string{"config.toml"},
		"path to one or more config files (will be merged in order)")
	flags.Bool("install", false, "run first-time DB installation")
	flags.Bool("upgrade", false, "upgrade the database schema to the current version")
	flags.Bool("version", false, "show current version")
	flags.Bool("yes", false, "assume 'yes' to prompts during install/upgrade")
	flags.String("idempotent", "", "make install idempotent (do not fail if already installed)")

	if err := flags.Parse(os.Args[1:]); err != nil {
		logger.Fatalf("error parsing flags: %v", err)
	}

	// Display version and exit.
	if ok, _ := flags.GetBool("version"); ok {
		fmt.Println(appVersion)
		os.Exit(0)
	}

	// Load configuration.
	ko := koanf.New(".")

	// Load config file(s).
	cFiles, _ := flags.GetStringSlice("config")
	for _, f := range cFiles {
		if err := ko.Load(file.Provider(f), toml.Parser()); err != nil {
			logger.Fatalf("error loading config from file %s: %v", f, err)
		}
	}

	// Load environment variables (prefixed with LISTMONK_).
	if err := ko.Load(env.Provider("LISTMONK_", ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, "LISTMONK_")), "__", ".", -1)
	}), nil); err != nil {
		logger.Fatalf("error loading config from env: %v", err)
	}

	// Load flags into koanf.
	if err := ko.Load(posflag.Provider(flags, ".", ko), nil); err != nil {
		logger.Fatalf("error loading config from flags: %v", err)
	}

	// Initialize the app.
	app := &App{
		log: logger,
		ko:  ko,
	}

	// Run install or upgrade if requested.
	if ok, _ := flags.GetBool("install"); ok {
		runInstall(app, flags)
		return
	}
	if ok, _ := flags.GetBool("upgrade"); ok {
		runUpgrade(app, flags)
		return
	}

	// Start the HTTP server.
	if err := initHTTPServer(app); err != nil {
		app.log.Fatalf("error starting HTTP server: %v", err)
	}
}
