//go:build !syslog
// +build !syslog

package main

import (
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/MindHunter86/NexusCloner/cloner"
	"github.com/rs/zerolog"
	"gopkg.in/urfave/cli.v1"
)

var log zerolog.Logger
var version = "devel" // -ldflags="-X 'main.version=X.X.X'"

func main() {
	app := cli.NewApp()
	app.Name = "NexusCloner"
	app.Version = version
	app.Compiled = time.Now()
	app.Authors = []cli.Author{
		{
			Name:  "Vadimka K.",
			Email: "admin@vkom.cc",
		},
	}
	app.Copyright = "(c) 2021 mindhunter86"
	app.Usage = "Repository cloning tool for nexus"

	cli.VersionFlag = cli.BoolFlag{
		Name:  "version, V",
		Usage: "print the version",
	}

	app.Flags = []cli.Flag{
		// Some common options
		cli.IntFlag{
			Name:  "verbose, v",
			Value: 4,
			Usage: "Verbose `LEVEL` (value from 5(debug) to 0(panic) and -1 for log disabling(quite mode))",
		},
		cli.BoolFlag{
			Name:  "quite, q",
			Usage: "Flag is equivalent to verbose -1",
		},
		cli.DurationFlag{
			Name:  "http-client-timeout",
			Usage: "Internal HTTP client connection `TIMEOUT` (format: 1000ms, 1s)",
			Value: 10 * time.Second,
		},
		cli.BoolFlag{
			Name:  "http-client-insecure",
			Usage: "Flag for TLS certificate verification disabling",
		},

		// Queue settings
		//

		// System settings
		cli.StringFlag{
			Name:  "temp-path-prefix",
			Usage: "Define prefix for temporary `directory`. If not defined, UNIX or WIN default will be used.",
		},
		cli.BoolFlag{
			Name:  "temp-path-save",
			Usage: "Flag for saving temp path content before program close. Flag for debugging only.",
		},

		// Application options
		cli.BoolFlag{
			Name:  "skip-download",
			Usage: "Skip download after finding missing assets. Flag for debugging only.",
		},
		cli.BoolFlag{
			Name:  "skip-download-errors",
			Usage: "Continue synchronization process if missing assets download detected",
		},
		cli.BoolFlag{
			Name:  "skip-upload",
			Usage: "Skip upload after downloading missing assets. Flag for debugging only.",
		},
		cli.StringFlag{
			Name:  "path-filter",
			Usage: "Regexp value with `path` for syncing.",
			Value: ".*",
		},
		cli.BoolFlag{
			Name:  "use-rpc",
			Usage: "Use RPC protocol for assets collection on source nexus server. Unstable, testing feature.",
		},
		cli.StringFlag{
			Name:  "rpc-endpoint",
			Usage: "Define RPC endpoint for assets collection",
			Value: "/service/extdirect",
		},
	}

	log := zerolog.New(zerolog.ConsoleWriter{
		Out: os.Stderr,
	}).With().Timestamp().Logger().Hook(SeverityHook{})
	zerolog.TimeFieldFormat = time.RFC3339Nano

	app.Action = func(c *cli.Context) (e error) {

		if c.Int("verbose") < -1 || c.Int("verbose") > 5 {
			log.Fatal().Msg("There is invalid data in verbose option. Option supports values for -1 to 5")
		}

		zerolog.SetGlobalLevel(zerolog.Level(int8((c.Int("verbose") - 5) * -1)))
		if c.Int("verbose") == -1 || c.Bool("quite") {
			zerolog.SetGlobalLevel(zerolog.Disabled)
		}

		return cloner.NewCloner(&log).Bootstrap(c) // Application starts here:
	}

	// sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))

	if e := app.Run(os.Args); e != nil {
		log.Fatal().Err(e).Msg("")
	}
}

type SeverityHook struct{}

func (h SeverityHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	if level != zerolog.DebugLevel {
		return
	}

	rfn := "unknown"
	pcs := make([]uintptr, 1)

	if runtime.Callers(4, pcs) != 0 {
		if fun := runtime.FuncForPC(pcs[0] - 1); fun != nil {
			rfn = fun.Name()
		}
	}

	fn := strings.Split(rfn, "/")
	e.Str("func", fn[len(fn)-1:][0])
}
