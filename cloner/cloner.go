package cloner

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"

	"github.com/rs/zerolog"
	"gopkg.in/urfave/cli.v1"
)

type Cloner struct {
	srcNexus, dstNexus *nexus
	mainDispatcher     *dispatcher
}

var (
	gLog     *zerolog.Logger
	gCli     *cli.Context
	gApi     *nexusApi
	gRpc     *rpcClient
	gQueue   *dispatcher
	gIsDebug bool
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

	if strings.ToLower(gCli.String("loglevel")) == "debug" {
		gIsDebug = true
	}

	var e error
	if m.srcNexus, e = newNexus().initiate(gCli.Args().Get(0)); e != nil {
		return e
	}

	if m.dstNexus, e = newNexus().initiate(gCli.Args().Get(1)); e != nil {
		return e
	}

	defer func() {
		m.srcNexus.destruct()
		m.dstNexus.destruct()
	}()

	// FEATURE QUEUE
	var kernSignal = make(chan os.Signal)
	signal.Notify(kernSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGQUIT)

	var wg sync.WaitGroup
	var ep = make(chan error, 1)

	m.mainDispatcher = newDispatcher(gCli.Int("queue-buffer"), gCli.Int("queue-workers-capacity"), gCli.Int("queue-workers"))
	gQueue = m.mainDispatcher

	go func() {
		wg.Add(1)
		defer wg.Done()

		gLog.Info().Msg("Main queue spawning ...")
		ep <- m.mainDispatcher.bootstrap()
	}()

	// FEATURE RPC
	if gCli.Bool("use-rpc") {
		gRpc = newRpcClient()
	}

	go func() {
		wg.Add(1)
		defer wg.Done()
		if err := m.srcNexus.getRepositoryAssetsRPC(gCli.String("path-filter")); err != nil {
			ep <- err
		}
		if err := m.dstNexus.getRepositoryAssetsRPC(gCli.String("path-filter")); err != nil {
			ep <- err
		}
	}()

LOOP:
	for {
		select {
		case <-kernSignal:
			gLog.Info().Msg("Syscall.SIG* has been detected! Closing application...")
			break LOOP
		case e = <-ep:
			if e != nil {
				gLog.Error().Err(e).Msg("Fatal Runtime Error!!! Abnormal application closing ...")
				break LOOP
			}
		}
	}

	m.mainDispatcher.destroy()
	wg.Wait()
	fmt.Println("OKOK")

	// return m.sync()
	return e
}

func (m *Cloner) syncRPC(srcNexus, dstNexus *nexus) (e error) {
	var missAssets []NexusAsset2
	if missAssets = m.getMissingAssetsRPC(srcNexus.assetsCollection, dstNexus.assetsCollection); len(missAssets) == 0 {
		return errClNoMissAssets
	}

	if gCli.Bool("skip-download") {
		return
	}

	if e = m.srcNexus.createTemporaryDirectory(); e != nil {
		return
	}

	if e = m.srcNexus.downloadMissingAssetsRPC(missAssets); e != nil {
		return
	}

	//
	// if e = m.srcNexus.downloadMissingAssets(missAssets); e != nil {
	// 	return
	// }

	// // 4. Uplaod missed assets
	// if gCli.Bool("skip-upload") {
	// 	return
	// }

	// m.dstNexus.setTemporaryDirectory(m.srcNexus.getTemporaryDirectory())
	// if e = m.dstNexus.uploadMissingAssets(missAssets); e != nil {
	// 	return
	// }

	return
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
		return // errClNoMissAssets
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

	// 4. Uplaod missed assets
	if gCli.Bool("skip-upload") {
		return
	}

	m.dstNexus.setTemporaryDirectory(m.srcNexus.getTemporaryDirectory())
	if e = m.dstNexus.uploadMissingAssets(missAssets); e != nil {
		return
	}

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

func (m *Cloner) getMissingAssetsRPC(srcCollection, dstCollection []NexusAsset2) (missAssets []NexusAsset2) {
	gLog.Debug().Int("srcColl", len(srcCollection)).Int("dstColl", len(dstCollection)).Msg("Starting search of missing assets")

	var dstAssets = make(map[string]NexusAsset2, len(dstCollection))
	for _, asset := range dstCollection {
		dstAssets[asset.getHumanReadbleName()] = asset
	}

	for _, asset := range srcCollection {
		if matched, _ := regexp.MatchString("((maven-metadata\\.xml)|\\.(md5|sha1|sha256|sha512))$", asset.getHumanReadbleName()); matched {
			gLog.Debug().Msgf("The asset %s will be skipped!", asset.getHumanReadbleName())
			continue
		}

		if _, found := dstAssets[asset.getHumanReadbleName()]; !found {
			missAssets = append(missAssets, asset)
		}
	}

	if gIsDebug {
		for _, asset := range missAssets {
			gLog.Debug().Msg("Missing asset - " + asset.getHumanReadbleName())
		}
	}

	gLog.Info().Msgf("There are %d missing assets in destination repository. Filelist u can see in debug logs.", len(missAssets))
	return
}

func (m *Cloner) getMissingAssets(srcACollection, dstACollection []*NexusAsset) (missingAssets []*NexusAsset) {
	var dstAssets = make(map[string]*NexusAsset, len(dstACollection))

	gLog.Debug().Int("srcColl", len(srcACollection)).Int("dstColl", len(dstACollection)).Msg("Starting search of missing assets")

	for _, asset := range dstACollection {
		dstAssets[asset.getHumanReadbleName()] = asset
	}

	for _, asset := range srcACollection {
		if matched, _ := regexp.MatchString("((maven-metadata\\.xml)|\\.(md5|sha1|sha256|sha512))$", asset.getHumanReadbleName()); matched {
			gLog.Debug().Msgf("The asset %s will be skipped!", asset.getHumanReadbleName())
			continue
		}

		if _, found := dstAssets[asset.getHumanReadbleName()]; !found {
			missingAssets = append(missingAssets, asset)
		}
	}

	if gIsDebug {
		for _, asset := range missingAssets {
			gLog.Debug().Msg("Missing asset - " + asset.getHumanReadbleName())
		}
	}

	gLog.Info().Msgf("There are %d missing assets in destination repository. Filelist u can see in debug logs.", len(missingAssets))
	return
}

// TODO CODE
// queue module

// TODO PLAN
// 1. get data from src and dst repos
// 2. compare dst assets from src (by id and checksum)
// 2.1 compare dst and src hashes
// 2.2 find missing assets on a filesystem (if tmp directory is exists) !! REVERT
// 2.3 check missing assets hashes with sums of files in tmp directory (if 2.2 is OK)
// 3. download assets from diff list
// 4. check checksum (md5)
// 5. upload verified assets to dst

// TODO 2
// CHECK getNexusFileMeta BUG with defer !!
