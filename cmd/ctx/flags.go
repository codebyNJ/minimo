package main

import "flag"

type cliFlags struct {
	config        string
	provider      string
	theme         string
	update        int
	version       bool
	help          bool
	noColor       bool
	debug         bool
	defaultConfig bool
	subcommand    string // "" (TUI), "status", or "stats"
	watch         bool   // status --watch
	json          bool   // status --json
}

// registerCommon wires the top-level flags onto fs, writing into f.
func registerCommon(fs *flag.FlagSet, f *cliFlags) {
	fs.StringVar(&f.config, "c", "", "path to config file")
	fs.StringVar(&f.config, "config", "", "path to config file")
	fs.BoolVar(&f.version, "V", false, "print version and exit")
	fs.BoolVar(&f.version, "version", false, "print version and exit")
	fs.BoolVar(&f.help, "h", false, "show help and exit")
	fs.BoolVar(&f.help, "help", false, "show help and exit")
	fs.IntVar(&f.update, "u", 0, "poll interval override in milliseconds")
	fs.IntVar(&f.update, "update", 0, "poll interval override in milliseconds")
	fs.StringVar(&f.provider, "provider", "", "restrict to a single provider")
	fs.StringVar(&f.theme, "theme", "", "color theme: default or mono")
	fs.BoolVar(&f.noColor, "no-color", false, "disable colored output")
	fs.BoolVar(&f.debug, "debug", false, "enable debug logging to ~/.ctx/ctx.log")
	fs.BoolVar(&f.defaultConfig, "default-config", false, "print default config YAML and exit")
}

// valueFlags consume the argument that follows them, so a bare "status" or
// "stats" immediately after one of these is that flag's value (e.g. a config
// path), not the subcommand.
var valueFlags = map[string]bool{
	"-c": true, "--config": true,
	"-u": true, "--update": true,
	"--provider": true,
	"--theme":    true,
}

func parseArgs(args []string) (cliFlags, error) {
	var f cliFlags

	// The subcommand is the first bare "status"/"stats" token wherever it
	// appears, so global flags may sit on either side of it
	// (e.g. `ctx -c cfg.yaml status --watch`). Everything else is handed to
	// the flag parser. Previously only args[0] was checked, so any leading
	// global flag caused the subcommand to be missed and the TUI to open.
	var rest []string
	prev := ""
	for _, a := range args {
		if f.subcommand == "" && (a == "status" || a == "stats") && !valueFlags[prev] {
			f.subcommand = a
			prev = a
			continue
		}
		rest = append(rest, a)
		prev = a
	}

	fs := flag.NewFlagSet("ctx", flag.ContinueOnError)
	registerCommon(fs, &f)
	if f.subcommand == "status" {
		fs.BoolVar(&f.watch, "watch", false, "re-render on changes")
		fs.BoolVar(&f.json, "json", false, "output JSON instead of a table")
	}
	if err := fs.Parse(rest); err != nil {
		return f, err
	}
	return f, nil
}
