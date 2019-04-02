package stats

import (
	. "0chain.net/logging"

	"go.uber.org/zap"
)

// NewWorker creates, and returns a new Worker object. Its only argument
// is a channel that the worker can add itself to whenever it is done its
// work.
func NewWorker(id int, workerQueue chan chan WorkRequest) *Worker {
	// Create, and return the worker.
	worker := Worker{
		ID:          id,
		Work:        make(chan WorkRequest),
		WorkerQueue: workerQueue,
		QuitChan:    make(chan bool)}

	return &worker
}

type Worker struct {
	ID          int
	Work        chan WorkRequest
	WorkerQueue chan chan WorkRequest
	QuitChan    chan bool
}

// This function "starts" the worker by starting a goroutine, that is
// an infinite "for-select" loop.
func (w *Worker) Start() {
	go func() {
		for {
			// Add ourselves into the worker queue.
			w.WorkerQueue <- w.Work

			select {
			case work := <-w.Work:
				// Receive a work request.
				Logger.Info("Received work", zap.Int("worker", w.ID), zap.Any("work", work))

				err := work.PerformWork()
				if err != nil {
					Logger.Error("Error in processing event", zap.Any("worker", w.ID), zap.Any("work", work), zap.Error(err))
				}
				Logger.Info("Completed work", zap.Int("worker", w.ID), zap.Any("work", work))

			case <-w.QuitChan:
				// We have been asked to stop.
				Logger.Info("Workder going to stop", zap.Int("worker", w.ID))
				return
			}
		}
	}()
}

// Stop tells the worker to stop listening for work requests.
//
// Note that the worker will only stop *after* it has finished its work.
func (w *Worker) Stop() {
	go func() {
		w.QuitChan <- true
	}()
}
