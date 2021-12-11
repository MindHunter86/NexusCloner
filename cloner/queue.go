package cloner

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	jobActParseAsset = uint8(iota)
	jobActGetAsset
	jobActFindAsset
	jobActDownloadAsset
	jobActUploadAsset
	jobActDeleteAsset
	jobActCustomFunc
)

const (
	jobStatusCreated = uint8(iota)
	jobStatusPending
	jobStatusBlocked
	jobStatusFailed
	jobStatusDone
)

const (
	wrkStatusCreated = uint8(iota)
	wrkStatusBusy
	wrkStatusWaiting
)

type (
	job struct {
		payload     []interface{}
		payloadFunc func([]interface{}) error
		fails       uint8

		mu             sync.RWMutex
		status, action uint8
	}
	jobError struct {
		err error
		job *job
	}
	worker struct {
		ctx context.Context

		pool  chan chan *job
		queue chan *job

		errors     chan *jobError
		statusPipe chan uint8
	}
	dispatcher struct {
		ctx    context.Context
		cancel context.CancelFunc

		queue     chan *job
		pool      chan chan *job
		errorPipe chan *jobError

		workers        int
		workerCapacity int
		workersQueue   chan *job

		statusPipe chan uint8
	}
	workerEvent struct {
		status uint8
		worker *worker
	}
)

func newDispatcher(queueBuffer, workerCapacity, workers int) *dispatcher {
	gLog.Debug().Msgf("Queue buf - %d ; Workers Capacity - %d ; Workers - %d ;", queueBuffer, workerCapacity, workers)
	ctx, cancel := context.WithCancel(context.Background())

	return &dispatcher{
		queue:     make(chan *job, queueBuffer),
		pool:      make(chan chan *job, workerCapacity),
		errorPipe: make(chan *jobError),

		workers:        workers,
		workerCapacity: workerCapacity,
		workersQueue:   make(chan *job, queueBuffer),

		statusPipe: make(chan uint8),

		ctx:    ctx,
		cancel: cancel,
	}
}

func (m *dispatcher) bootstrap() (e error) {
	gLog.Debug().Msg("dispatcher initialization...")

	var wg sync.WaitGroup
	wg.Add(m.workers)

	for i := 0; i < m.workers; i++ {
		go func(num int) {
			gLog.Debug().Msgf("Spawning worker #%d ...", num)
			newWorker(m).spawn(num)
			gLog.Debug().Msgf("Worker #%d died", num)
			wg.Done()
		}(i)
	}

	gLog.Debug().Msg("workers spawned successfully")
	m.dispatch()

	fmt.Println("WAIT")
	wg.Wait()
	fmt.Println("OK")
	return
}

func (m *dispatcher) dispatch() {
	gLog.Debug().Msg("dispatcher start dispatching...")
	gLog.Debug().Msg("dispatcher - entering in event loop")

	var mu sync.RWMutex
	var waitingWorkers int

	var timer = time.NewTimer(5 * time.Second)

	for {
		select {

		case <-m.ctx.Done():
			gLog.Debug().Msg("dispatcher stopping dispatching...")
			// for len(m.workersQueue) != 0 {
			// 	<-m.workersQueue
			// }
			// for len(m.queue) != 0 {
			// 	<-m.queue
			// }
			gLog.Debug().Msg("dispatcher stopped")
			return
		// case j := <-m.queue:
		// 	m.workersQueue <- j
		// go func() {
		// defer func() {
		// 	if recover() != nil {
		// 		gLog.Warn().Msg("Panic caught! There is task loss detected!!!")
		// 	}
		// }()

		// for {
		// 	select {
		// 	case <-m.ctx.Done():
		// 		return
		// 	case m.workersQueue <- j:
		// 	}
		// }
		// }(Reset changes the timer to expire after duration d. It returns true if the timer had been active, false if the timer had expired or been stopped.)
		case status := <-m.statusPipe:
			if timer.Stop() {
				gLog.Info().Msg("Timer stopped")
			}

			switch status {
			case wrkStatusBusy:
				mu.Lock()
				waitingWorkers = waitingWorkers - 1
				mu.Unlock()
			case wrkStatusWaiting:
				mu.Lock()
				waitingWorkers = waitingWorkers + 1
				mu.Unlock()
			}

			gLog.Debug().Msgf("STATUS active - %d;", waitingWorkers)
			if waitingWorkers == m.workers {
				gLog.Info().Msg("Reset timer")
				timer.Reset(5 * time.Second)
			}
		case <-timer.C:
			gLog.Info().Msg("There is no jobs, closing dispatcher")
			m.cancel()
			return

		case jErr := <-m.errorPipe:
			if jErr.job.fails != 3 {
				gLog.Warn().Uint8("fails", jErr.job.fails).Err(jErr.err).Msg("There is failed job found! Trying to restart task.")
				m.queue <- jErr.job
			} else {
				gLog.Error().Err(jErr.err).Msg("There is failed job found with unsuccessful retries! Skipping this task ...")
			}
		}
	}
}

func (m *dispatcher) getQueueChan() chan *job {
	return m.queue
}

func (m *dispatcher) destroy() {
	gLog.Debug().Msg("Send STOP to all workers...")
	m.cancel()
}

func (m *dispatcher) newJob(j *job) {
	for {
		select {
		case <-m.ctx.Done():
			return
		case m.queue <- j:
			return
		}
	}
}

func newWorker(dp *dispatcher) *worker {
	return &worker{
		ctx:    dp.ctx,
		errors: dp.errorPipe,
		// queue:  dp.workersQueue,
		queue:      dp.queue,
		statusPipe: dp.statusPipe,
	}
}

func (m *worker) spawn(i int) {
	gLog.Debug().Msgf("Worker #%d has been spawned!", i)
	m.setStatus(wrkStatusWaiting)

	for {
		select {
		case <-m.ctx.Done():
			gLog.Debug().Msgf("Worker #%d received STOP signal. Stopping...", i)
			return
		case j := <-m.queue:
			m.setStatus(wrkStatusBusy)
			j.setStatus(jobStatusPending)
			m.doJob(j)
			m.setStatus(wrkStatusWaiting)
		}
	}
}

func (m *worker) setStatus(status uint8) {
	for {
		select {
		case <-m.ctx.Done():
			return
		case m.statusPipe <- status:
			return
		}
	}
}

func (m *worker) doJob(j *job) {
	switch j.action {
	case jobActParseAsset:
		nxs := j.payload[0].(*nexus)
		if e := nxs.getRepositoryAssetsRPC(j.payload[1].(string)); e != nil {
			m.errors <- j.newError(e)
			return
		}
	case jobActGetAsset:
		nxs := j.payload[0].(*nexus)
		if e := nxs.getRepositoryAssetInfo(j.payload[1].(NexusAsset2)); e != nil {
			m.errors <- j.newError(e)
			return
		}
	case jobActFindAsset:
	case jobActDownloadAsset:
		nxsFrom := j.payload[0].(*nexus)
		asset := j.payload[1].(NexusAsset2)
		nxsTo := j.payload[2].(*nexus)

		if e := nxsFrom.downloadAssetRPC(asset); e != nil {
			m.errors <- j.newError(e)
			return
		}

		nxsFrom.incDownloadedAssets()
		gLog.Info().Msgf("Asset %s has been downloaded successfully.", asset.getHumanReadbleName())

		if gCli.Bool("skip-upload") {
			gLog.Warn().Msg("Skipping asset uploading because of --skip-upload flag received")
			return
		}

		gQueue.newJob(&job{
			action:  jobActUploadAsset,
			payload: []interface{}{nxsTo, asset},
		})
	case jobActUploadAsset:
		nxs := j.payload[0].(*nexus)
		asset := j.payload[1].(NexusAsset2)

		if gCli.Bool("use-put-upload") {
			if e := nxs.uploadAssetHttpFormatRPC(asset); e != nil {
				m.errors <- j.newError(e)
				return
			}
		} else {
			if e := nxs.uploadAssetRPC(asset); e != nil {
				m.errors <- j.newError(e)
				return
			}
		}

		nxs.incUploadedAssets()
		gLog.Info().Msgf("Asset %s has been upload successfully.", asset.getHumanReadbleName())

		gQueue.newJob(&job{
			action:  jobActDeleteAsset,
			payload: []interface{}{nxs, asset},
		})
	case jobActDeleteAsset:
		nxs := j.payload[0].(*nexus)
		asset := j.payload[1].(NexusAsset2)

		if e := nxs.deleteAssetTemporaryFile(asset); e != nil {
			m.errors <- j.newError(e)
			return
		}

		// case jobActCustomFunc:
		// 	if j.payloadFunc != nil {
		// 		if e := j.payloadFunc(j.payload); e != nil {
		// 			gLog.Error().Err(e).Msg("There is some troubles in task exec!")
		// 			m.errors <- j.newError(e)
		// 		}
		// 	} else {
		// 		gLog.Error().Msg("Given job has invalid status! Skipping...")
		// 	}
	}
}

func (m *job) setStatus(status uint8) {
	m.mu.Lock()
	if status == jobStatusFailed {
		m.fails++
	}

	m.status = status
	m.mu.Unlock()
}

func (m *job) newError(err error) *jobError {
	m.setStatus(jobStatusFailed)
	return &jobError{
		err: err,
		job: m,
	}
}
