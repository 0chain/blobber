package seqpriorityqueue

import (
	"container/heap"
	"sync"
)

type UploadData struct {
	Offset    int64
	DataBytes int64
	IsFinal   bool
}

type queue []UploadData

func (pq queue) Len() int { return len(pq) }

func (pq queue) Less(i, j int) bool {
	return pq[i].Offset < pq[j].Offset
}

func (pq queue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *queue) Push(x interface{}) {
	*pq = append(*pq, x.(UploadData))
}

func (pq *queue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

// SeqPriorityQueue is a priority queue that pops items in sequential order that starts from 0
type SeqPriorityQueue struct {
	queue    queue
	lock     sync.Mutex
	cv       *sync.Cond
	next     int64
	done     bool
	dataSize int64
}

// NewSeqPriorityQueue creates a new SequentialPriorityQueue
func NewSeqPriorityQueue(dataSize int64) *SeqPriorityQueue {
	pq := &SeqPriorityQueue{
		queue:    make(queue, 0),
		done:     false,
		dataSize: dataSize,
	}
	pq.cv = sync.NewCond(&pq.lock)
	heap.Init(&pq.queue)

	return pq
}

func (pq *SeqPriorityQueue) Push(v UploadData) {
	pq.lock.Lock()
	if v.Offset >= pq.next {
		heap.Push(&pq.queue, v)
		if v.Offset == pq.next {
			pq.cv.Signal()
		}
	}
	pq.lock.Unlock()
}

func (pq *SeqPriorityQueue) Done(v UploadData) {
	pq.lock.Lock()
	pq.done = true
	heap.Push(&pq.queue, v)
	pq.cv.Signal()
	pq.lock.Unlock()
}

func (pq *SeqPriorityQueue) Popup() UploadData {
	pq.lock.Lock()
	for pq.queue.Len() == 0 && !pq.done || (pq.queue.Len() > 0 && pq.queue[0].Offset != pq.next) {
		pq.cv.Wait()
	}
	if pq.done && pq.dataSize > 0 {
		pq.lock.Unlock()
		return UploadData{
			Offset:    pq.next,
			DataBytes: pq.dataSize - pq.next,
		}
	}
	retItem := UploadData{
		Offset: pq.next,
	}
	for pq.queue.Len() > 0 && pq.queue[0].Offset == pq.next {
		item := heap.Pop(&pq.queue).(UploadData)
		pq.next += item.DataBytes
	}
	retItem.DataBytes = pq.next - retItem.Offset
	retItem.IsFinal = pq.done
	pq.lock.Unlock()
	return retItem
}
