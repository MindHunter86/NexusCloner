package cloner

import (
	"github.com/rs/zerolog"
	"gopkg.in/urfave/cli.v1"
)

type Cloner struct {
	srcNexus, dstNexus *nexus
}

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

	m.srcNexus = newNexus(
		gCli.String("srcRepoUrl"),
		gCli.String("srcRepoUsername"),
		gCli.String("srcRepoPassword"),
		gCli.String("srcRepoName"),
	)

	m.dstNexus = newNexus(
		gCli.String("dstRepoUrl"),
		gCli.String("dstRepoUsername"),
		gCli.String("dstRepoPassword"),
		gCli.String("dstRepoName"),
	)

	// _, err := newNexus().getRepositoryAssets(gCli.String("srcRepoName"))
	return nil
}

func (m *Cloner) getMetaFromRepositories() {

}

// TODO
// 1. get data from src and dst repos
// 2. compare dst assets from src (by path and checksum)
// 3. download assets from diff list
// 4. check checksum (md5)
// 5. upload verified assets to dst
