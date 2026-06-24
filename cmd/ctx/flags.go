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
	subcommand    string // "" (TUI) or "status"
	watch         bool   // status --watch
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

func parseArgs(args []string) (cliFlags, error) {
	var f cliFlags

	// Subcommand is the first non-flag argument when it is "status".
	rest := args
	if len(args) > 0 && args[0] == "status" {
		f.subcommand = "status"
		rest = args[1:]
	}

	fs := flag.NewFlagSet("ctx", flag.ContinueOnError)
	registerCommon(fs, &f)
	if f.subcommand == "status" {
		fs.BoolVar(&f.watch, "watch", false, "re-render on changes")
	}
	if err := fs.Parse(rest); err != nil {
		return f, err
	}
	return f, nil
}
