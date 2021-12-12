package cloner

import (
	"errors"
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
	uplDispatcher      *dispatcher
}

var (
	gLog      *zerolog.Logger
	gCli      *cli.Context
	gApi      *nexusApi
	gRpc      *rpcClient
	gQueue    *dispatcher
	gUplQueue *dispatcher
	gIsDebug  bool
)

var (
	errClNoMissAssets = errors.New("There is no missing assets detected. Repository synchronization is not needed.")
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
	var ep = make(chan error, 10) // !!

	m.mainDispatcher = newDispatcher(gCli.Int("queue-buffer"), gCli.Int("queue-workers-capacity"), gCli.Int("queue-workers"))
	gQueue = m.mainDispatcher

	go func() {
		wg.Add(1)
		defer wg.Done()

		gLog.Info().Msg("Main queue spawning ...")
		ep <- m.mainDispatcher.bootstrap(false)
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

	var fuckyou bool
LOOP:
	for {
		select {
		case <-kernSignal:
			gLog.Info().Msg("Syscall.SIG* has been detected! Closing application...")
			break LOOP
		case e = <-ep:
			if fuckyou {
				continue
			}
			if e != nil {
				gLog.Error().Err(e).Msg("Fatal Runtime Error!!! Abnormal application closing ...")
				break LOOP
			}

			gLog.Info().Msgf("Found %d assets in src and %d in dst", len(m.srcNexus.assetsCollection), len(m.dstNexus.assetsCollection))

			if e = m.syncRPC(ep); e != nil {
				break LOOP
			}

			gLog.Info().Msg("GOOD, CLOSE APPLICATION")
			fuckyou = true
		}
	}

	m.mainDispatcher.destroy()
	m.uplDispatcher.destroy()
	wg.Wait()

	// return m.sync()
	return e
}

func (m *Cloner) syncRPC(ep chan error) (e error) {
	var missAssets []NexusAsset2
	if missAssets = m.getMissingAssetsRPC(m.srcNexus.assetsCollection, m.dstNexus.assetsCollection); len(missAssets) == 0 {
		return errClNoMissAssets
	}

	// if gCli.Bool("skip-download") {
	// 	return
	// }

	// dispathcer restart
	var wg sync.WaitGroup
	m.mainDispatcher = newDispatcher(gCli.Int("queue-buffer"), gCli.Int("queue-workers-capacity"), gCli.Int("queue-workers"))
	gQueue = m.mainDispatcher

	m.uplDispatcher = newDispatcher(gCli.Int("queue-buffer"), gCli.Int("queue-workers-capacity"), gCli.Int("queue-workers"))
	gUplQueue = m.uplDispatcher

	wg.Add(1) // !!
	go func() {
		defer wg.Done()

		gLog.Info().Msg("Main queue spawning ...")
		ep <- m.mainDispatcher.bootstrap(false)
	}()
	go func() {
		// defer wg.Done()

		gLog.Info().Msg("Upload queue spawning ...")
		ep <- m.uplDispatcher.bootstrap(true)
	}()

	if e = m.srcNexus.createTemporaryDirectory(); e != nil {
		return
	}
	m.dstNexus.setTemporaryDirectory(m.srcNexus.getTemporaryDirectory())

	if e = m.srcNexus.downloadMissingAssetsRPC(missAssets, m.dstNexus); e != nil {
		return
	}

	wg.Wait()
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
		if matched, _ := regexp.MatchString("((maven-metadata\\.xml)|\\.(pom|md5|sha1|sha256|sha512))$", asset.getHumanReadbleName()); matched {
			gLog.Debug().Msgf("The asset %s will be skipped!", asset.getHumanReadbleName())
			continue
		}

		if _, found := dstAssets[asset.getHumanReadbleName()]; !found {
			missAssets = append(missAssets, asset)
			continue
		}

		if !gCli.Bool("skip-verify-hashes") {
			var e error
			var sHashes, dHashes map[string]string

			if sHashes, e = asset.getHashes(); e != nil {
				gLog.Error().Err(e).Msg("There is some problems with taking source asset hashes!")

				if gCli.Bool("verify-errors-ignore") {
					missAssets = append(missAssets, asset)
					continue
				}
			}

			if dHashes, e = dstAssets[asset.getHumanReadbleName()].getHashes(); e != nil {
				gLog.Error().Err(e).Msg("There is some problems with taking dest asset hashes!")

				if gCli.Bool("verify-errors-ignore") {
					missAssets = append(missAssets, asset)
					continue
				}
			}

			if len(sHashes) == 0 || len(dHashes) == 0 {
				gLog.Warn().Msg("There are no hashes for given asset! Are hashes are enabled on Nexus instances?")

				if gCli.Bool("verify-errors-ignore") {
					missAssets = append(missAssets, asset)
					continue
				}
			}

			var matched bool
			switch {
			// case sHashes["md5"] == dHashes["md5"]:
			// 	gLog.Debug().Str("hash", "md5").Msgf("ASSET OK %s - %s", sHashes["md5"], dHashes["md5"])
			// 	matched = true
			case sHashes["sha1"] == dHashes["sha1"]:
				gLog.Debug().Str("hash", "sha1").Msgf("ASSET OK %s - %s", sHashes["sha1"], dHashes["sha1"])
				matched = true
			case sHashes["sha256"] == dHashes["sha256"]:
				gLog.Debug().Str("hash", "sha256").Msgf("ASSET OK %s - %s", sHashes["sha256"], dHashes["sha256"])
				matched = true
			case sHashes["sha512"] == dHashes["sha512"]:
				gLog.Debug().Str("hash", "sha512").Msgf("ASSET OK %s - %s", sHashes["sha512"], dHashes["sha512"])
				matched = true
			}

			if matched {
				continue
			}

			gLog.Debug().Msgf("There is no matched hashes for asset %s. Tasking for file rewriting.", asset.getHumanReadbleName())
			missAssets = append(missAssets, asset)
			continue
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
