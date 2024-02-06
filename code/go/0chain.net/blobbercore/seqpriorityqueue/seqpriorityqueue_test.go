package seqpriorityqueue

import (
	"testing"
	"time"
)

func TestSeqPriorityQueue(t *testing.T) {
	pq := NewSeqPriorityQueue(21)

	go func() {
		for i := 19; i >= 0; i-- {
			j := i
			go func() {
				ud := UploadData{
					Offset:    int64(j),
					DataBytes: 1,
				}
				pq.Push(ud)
			}()
		}
		time.Sleep(100 * time.Millisecond)
		pq.Done(UploadData{
			Offset:    20,
			DataBytes: 1,
		})
	}()
	expectedOffset := int64(0)
	for {
		ud := pq.Popup()
		if ud.Offset != expectedOffset {
			t.Errorf("expected offset %v, got %v", expectedOffset, ud.Offset)
		}
		if ud.DataBytes == 0 {
			if expectedOffset+(21-ud.Offset) != 21 {
				t.Errorf("expected 21, got %v", expectedOffset+(21-ud.Offset))
			}
			break
		}
		expectedOffset += ud.DataBytes
	}

}
