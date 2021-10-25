package main

import (
	"log/syslog"
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

func main() {
	zerolog.TimeFieldFormat = time.RFC3339Nano

	log = zerolog.New(zerolog.ConsoleWriter{
		Out: os.Stderr,
	}).With().Timestamp().Logger()

	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	log = log.Hook(SeverityHook{})

	app := cli.NewApp()
	app.Name = "NexusCloner"
	app.Version = "0.1"
	app.Compiled = time.Now()
	app.Authors = []cli.Author{
		{
			Name:  "Vadimka K.",
			Email: "admin@vkom.cc",
		},
	}
	app.Copyright = "(c) 2021 mindhunter86"
	app.Usage = "Repository cloning tool for nexus"

	app.Flags = []cli.Flag{
		// Some common options
		cli.StringFlag{
			Name:  "loglevel, l",
			Value: "debug",
			Usage: "log level (debug, info, warn, error, fatal, panic)",
		},
		cli.StringFlag{
			Name:  "syslog-proto",
			Value: "tcp",
		},
		cli.StringFlag{
			Name:  "syslog-server",
			Value: "",
			Usage: "DON'T FORGET ABOUT TLS\\SSL, COMRADE",
		},
		cli.StringFlag{
			Name:  "syslog-tag",
			Value: "",
		},
		cli.DurationFlag{
			Name:  "http-client-timeout",
			Usage: "internal HTTP client timeout (ms)",
			Value: 1000 * time.Millisecond,
		},
		cli.BoolFlag{
			Name:  "http-client-insecure",
			Usage: "disable TLS certificate verification",
		},

		// Queue settings
		//

	if len(m.tempPath) != 0 && !gCli.Bool("temp-path-save") {

		// System settings
		cli.StringFlag{
			Name:  "temp-path-prefix",
			Usage: "Define prefix for temporary directory. If not defined, UNIX or WIN default will be used.",
		},
		cli.BoolFlag{
			Name: "temp-path-save",
			Usage: "Flag for saving temp path content before program close. Flag for debugging only.",
		},

		// Application options
		cli.StringFlag{
			Name:  "tmpdir",
			Usage: "temporary directory for artefacts synchronization",
		},
		cli.StringFlag{
			Name:  "srcRepoName",
			Usage: "Source repository name",
		},
		cli.StringFlag{
			Name:  "srcRepoUrl",
			Usage: "Source repository url",
		},
		cli.StringFlag{
			Name:  "srcRepoUsername",
			Usage: "Credentials for source repository access",
		},
		cli.StringFlag{
			Name:  "srcRepoPassword",
			Usage: "Credentials for source repository access",
		},
		cli.StringFlag{
			Name:  "dstRepoName",
			Usage: "Destination repository name",
		},
		cli.StringFlag{
			Name:  "dstRepoUrl",
			Usage: "Destination repository url",
		},
		cli.StringFlag{
			Name:  "dstRepoUsername",
			Usage: "Credentials for destination repository access",
		},
		cli.StringFlag{
			Name:  "dstRepoPassword",
			Usage: "Credentials for destination repository access",
		},
	}
	app.Action = func(c *cli.Context) (e error) {
		log.Debug().Msg("prgm started")

		// Usage: "log level (debug, info, warn, error, fatal, panic)",
		switch c.String("loglevel") {
		case "debug":
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		case "info":
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
		case "warn":
			zerolog.SetGlobalLevel(zerolog.WarnLevel)
		case "error":
			zerolog.SetGlobalLevel(zerolog.ErrorLevel)
		case "fatal":
			zerolog.SetGlobalLevel(zerolog.FatalLevel)
		case "panic":
			zerolog.SetGlobalLevel(zerolog.PanicLevel)
		default:
			log.Warn().Str("input", c.String("loglevel")).Msg("Abnormal data has been given for loglevel!")
		}

		var sLog *syslog.Writer
		if len(c.String("syslog-server")) != 0 {
			log.Debug().Msg("Connecting to syslog server ...")

			if sLog, e = syslog.Dial(
				c.String("syslog-proto"),
				c.String("syslog-server"),
				syslog.LOG_INFO, // 2do put it into args
				c.String("syslog-tag"),
			); e != nil {
				return
			}

			log.Debug().Msg("Syslog connection established! Reset zerolog for MultiLevelWriter set ...")

			log = zerolog.New(zerolog.MultiLevelWriter(
				zerolog.ConsoleWriter{
					Out: os.Stderr,
				},
				sLog,
			)).With().Timestamp().Logger()

			log = log.Hook(SeverityHook{})

			log.Info().Msg("Zerolog reinitialized! Starting commands...")
		}

		// Application starts here:
		return cloner.NewCloner(&log).Bootstrap(c)
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))

	if e := app.Run(os.Args); e != nil {
		log.Fatal().Err(e).Msg("")
	}
}

type SeverityHook struct{}

func (h SeverityHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
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
