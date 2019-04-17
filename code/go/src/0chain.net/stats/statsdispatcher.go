package stats

import (
	"go.uber.org/zap"

	. "0chain.net/logging"
)

type WorkRequest interface {
	PerformWork() error
}

// A buffered channel that we can send work requests on.
var WorkQueue = make(chan WorkRequest, 100)

var WorkerQueue chan chan WorkRequest

func AddNewChallengeEvent(allocationID string, challengeID string) {
	event := &ChallengeEvent{AllocationID: allocationID, ChallengeID: challengeID, Result: NEW, RedeemStatus: NOTREDEEMED}
	AddStatsEvent(event)
}

func AddChallengeProcessedEvent(allocationID string, challengeID string, result ChallengeStatus) {
	event := &ChallengeEvent{AllocationID: allocationID, ChallengeID: challengeID, Result: result, RedeemStatus: NOTREDEEMED}
	AddStatsEvent(event)
}

func AddChallengeRedeemedEvent(allocationID string, challengeID string, result ChallengeStatus, redeemStatus ChallengeRedeemStatus, path string, redeemTxn string) {
	event := &ChallengeEvent{AllocationID: allocationID, ChallengeID: challengeID, Result: result, RedeemStatus: redeemStatus, Path: path, RedeemTxn: redeemTxn}
	AddStatsEvent(event)
}

func AddNewAllocationEvent(allocationID string) {
	event := &AllocationEvent{AllocationID: allocationID}
	AddStatsEvent(event)
}

func AddFileUploadedStatsEvent(allocationID string, path string, wmKey string, size int64) {
	event := &FileUploadedEvent{AllocationID: allocationID, Path: path, WriteMarkerKey: wmKey, Size: size, Operation: INSERT_UPDATE_OPERATION}
	AddStatsEvent(event)
}

func AddFileDeletedStatsEvent(allocationID string, path string, wmKey string, size int64) {
	event := &FileUploadedEvent{AllocationID: allocationID, Path: path, WriteMarkerKey: wmKey, Size: -size, Operation: DELETE_OPERATION}
	AddStatsEvent(event)
}

func AddBlockDownloadedStatsEvent(allocationID string, path string) {
	event := &FileDownloadedEvent{AllocationID: allocationID, Path: path}
	AddStatsEvent(event)
}

func AddStatsEvent(req WorkRequest) {
	WorkQueue <- req
	Logger.Info("Stats Event added", zap.Any("event", req))
}

var workers []*Worker

func StartEventDispatcher(nworkers int) {
	// First, initialize the channel we are going to but the workers' work channels into.
	WorkerQueue = make(chan chan WorkRequest, nworkers)
	workers = make([]*Worker, nworkers)

	// Now, create all of our workers.
	for i := 0; i < nworkers; i++ {
		Logger.Info("Starting worker", zap.Int("worker", i+1))
		worker := NewWorker(i+1, WorkerQueue)
		worker.Start()
		workers = append(workers, worker)
	}

	go func() {
		for {
			select {
			case work := <-WorkQueue:
				Logger.Info("Received work requeust")
				go func() {
					worker := <-WorkerQueue

					Logger.Info("Dispatching work request")
					worker <- work
				}()
			}
		}
	}()
}

func StopEventDispatcher() {
	for _, worker := range workers {
		worker.Stop()
	}
}
