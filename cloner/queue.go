package cloner

import "sync"

const (
	jobActParseAsset = uint8(iota)
	jobActGetAsset
	jobActDownloadAsset
	jobActUploadAsset
	jobActCustomFunc
)

const (
	jobStatusCreated = uint8(iota)
	jobStatusPending
	jobStatusBlocked
	jobStatusFailed
	jobStatusDone
)

type (
	job struct {
		payload        []interface{}
		payloadFunc    func([]interface{}) error
		status, action uint8
		fails          uint8
	}
	jobError struct {
		err error
		job *job
	}
	worker struct {
		pool  chan chan *job
		inbox chan *job

		done   chan struct{}
		errors chan *jobError
	}
	dispatcher struct {
		queue      chan *job
		pool       chan chan *job
		done       chan struct{}
		workerDone chan struct{}
		errorPipe  chan *jobError

		workerCapacity int
	}
)

func newDispatcher(queueBuffer, workerCapacity int) *dispatcher {
	return &dispatcher{
		queue:      make(chan *job, queueBuffer),
		pool:       make(chan chan *job, workerCapacity),
		done:       make(chan struct{}, 1),
		workerDone: make(chan struct{}, 1),
		errorPipe:  make(chan *jobError),

		workerCapacity: workerCapacity,
	}
}

func newWorker(dp *dispatcher) *worker {
	return &worker{
		pool:   dp.pool,
		inbox:  make(chan *job, dp.workerCapacity),
		done:   dp.workerDone,
		errors: dp.errorPipe,
	}
}

func newJob() *job {
	return &job{
		status: jobStatusCreated,
	}
}

func (m *dispatcher) bootstrap(workers int) (e error) {
	gLog.Debug().Msg("dispatcher initialization...")

	var wg sync.WaitGroup
	wg.Add(workers + 1)

	for i := 0; i < workers; i++ {
		go func(wg sync.WaitGroup) {
			newWorker(m).spawn()
			wg.Done()
		}(wg)
	}

	go func(wg sync.WaitGroup) {
		m.dispatch()
		close(m.workerDone)
		wg.Done()
	}(wg)

	gLog.Debug().Msg("workers spawned successfully")
	gLog.Debug().Msg("dispatcher spawner in waiting mode")
	wg.Wait()

	return
}

func (m *dispatcher) dispatch() {
	gLog.Debug().Msg("dispatcher start dispatching...")
	gLog.Debug().Msg("dispatcher - entering in event loop")

	for {
		select {
		case <-m.done:
			return
		case j := <-m.queue:
			go func(j *job) {
				nextWorker := <-m.pool
				nextWorker <- j
			}(j)
		case jErr := <-m.errorPipe:
			if jErr.job.fails != 3 {
				gLog.Warn().Msg("There is failed job found! Trying to restart task.")
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
	close(m.done)
}

func (m *worker) spawn() {
	defer close(m.inbox)

	for {
		m.pool <- m.inbox
		select {
		case <-m.done:
		case j := <-m.inbox:
			j.setStatus(jobStatusPending)
			m.doJob(j)
		}
	}
}

func (m *worker) doJob(j *job) {
	switch j.action {
	case jobActParseAsset:
		nexus := j.payload[0].(*nexus)
		if e := nexus.getRepositoryAssetsRPC(j.payload[1].(string)); e != nil {
			m.errors <- j.newError(e)
		}
	case jobActGetAsset:
	case jobActDownloadAsset:
	case jobActUploadAsset:
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
	if status == jobStatusFailed {
		m.fails++
	}

	m.status = status
}

func (m *job) newError(err error) *jobError {
	m.setStatus(jobStatusFailed)
	return &jobError{
		err: err,
		job: m,
	}
}
