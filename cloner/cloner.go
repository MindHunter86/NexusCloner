package cloner

import (
	"github.com/rs/zerolog"
	"gopkg.in/urfave/cli.v1"
)

type Cloner struct{}

var (
	gLog *zerolog.Logger
	gCli *cli.Context
	gApi *nexusApi
)

func NewCloner(l *zerolog.Logger) *Cloner {
	gLog = l
	return &Cloner{}
}

func (m *Cloner) Bootstrap(ctx *cli.Context) error {
	gCli = ctx
	gApi = NewNexusApi()

	_, err := NewNexus().getRepositoryAssets(gCli.String("srcRepoName"))
	return err
}
