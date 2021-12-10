package cloner

import "context"

type (
	xQueue struct {
		name   string
		jobs   chan *xJob
		ctx    context.Context
		cancel context.CancelFunc
	}
	xJob struct {
		name    string
		payload []interface{}
		action  func([]interface{}) error
	}
	xWorker struct {
		queue *xQueue
	}
)

func newXQueue(name string) *xQueue {
	ctx, cancel := context.WithCancel(context.Background())

	return &xQueue{
		jobs:   make(chan *xJob),
		name:   name,
		ctx:    ctx,
		cancel: cancel,
	}
}

func newXWorker(queue *xQueue) *xWorker {
	return &xWorker{
		queue: queue,
	}
}

func (m *xQueue) addJob(job *xJob) {
	m.jobs <- job
}

func (m *xJob) run() error {
	return m.action(m.payload)
}

func (m *xWorker) DoWork() bool {
	for {
		select {
		case <-m.queue.ctx.Done():
			return true
		case job := <-m.queue.jobs:
			e := job.run()
			gLog.Error().Err(e).Msg("There is some errors while task exec.")
		}
	}
}
