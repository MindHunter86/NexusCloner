package cloner

import (
	"errors"
	"strings"

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

var (
	errClNoMissAssets = errors.New("There is no missing assets detected. Repository sinchronization is not needed.")
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

	defer func() {
		m.srcNexus.destruct()
		m.dstNexus.destruct()
	}()

	return m.sync()
	// _, err := newNexus().getRepositoryAssets(gCli.String("srcRepoName"))
}

func (m *Cloner) sync() (e error) {

	// 1. get data from src and dst repositories
	var srcAssets, dstAssets []*NexusAsset
	if srcAssets, dstAssets, e = m.getMetaFromRepositories(); e != nil {
		return
	}

	// 2. compare  dst assets from src (by id)
	// TODO - compare by checksum if flag found (2.1)
	var missAssets []*NexusAsset
	if missAssets = m.getMissingAssets(srcAssets, dstAssets); len(missAssets) == 0 {
		return errClNoMissAssets
	}

	// 3. download missed assets from src repository
	if gCli.Bool("skip-download") {
		return
	}

	if e = m.srcNexus.createTemporaryDirectory(); e != nil {
		return
	}

	if e = m.srcNexus.downloadMissingAssets(missAssets); e != nil {
		return
	}

	// var tmpdir string
	// tmpdir = m.srcNexus.getTemporaryDirectory()

	return
}

func (m *Cloner) getMetaFromRepositories() (srcAssets, dstAssets []*NexusAsset, e error) {
	if srcAssets, e = m.srcNexus.getRepositoryAssets(); e != nil {
		return
	}

	if dstAssets, e = m.dstNexus.getRepositoryAssets(); e != nil {
		return
	}

	return
}

func (m *Cloner) getMissingAssets(srcACollection, dstACollection []*NexusAsset) (missingAssets []*NexusAsset) {
	var dstAssets = make(map[string]*NexusAsset, len(dstACollection))

	gLog.Debug().Int("srcColl", len(srcACollection)).Int("dstColl", len(dstACollection)).Msg("Starting search of missing assets")

	for _, asset := range dstACollection {
		dstAssets[strings.ReplaceAll(asset.Path, "/", "_")] = asset
	}

	for _, asset := range srcACollection {
		if _, found := dstAssets[strings.ReplaceAll(asset.Path, "/", "_")]; !found {
			missingAssets = append(missingAssets, asset)
		}
	}

	for _, asset := range missingAssets {
		gLog.Debug().Msg("Missing asset - " + strings.ReplaceAll(asset.Path, "/", "_"))
	}

	gLog.Info().Msgf("There are %d missing assets in destination repository", len(missingAssets))
	return
}

// TODO CODE
// queue module

// TODO PLAN
// 1. get data from src and dst repos
// 2. compare dst assets from src (by id and checksum)
// 2.1 compare dst and src hashes
// 3. download assets from diff list
// 4. check checksum (md5)
// 5. upload verified assets to dst
