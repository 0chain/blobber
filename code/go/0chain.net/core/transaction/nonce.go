package transaction

import (
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/zcncore"
	"go.uber.org/zap"
	"sync"
)

var monitor = &nonceMonitor{
	failed:                   map[int64]int64{},
	used:                     map[int64]struct{}{},
	highestSuccess:           0,
	shouldRefreshFromBalance: true,
}

type nonceMonitor struct {
	sync.Mutex
	failed                   map[int64]int64
	used                     map[int64]struct{}
	highestSuccess           int64
	shouldRefreshFromBalance bool
}

func (m *nonceMonitor) getNextUnusedNonce() int64 {
	m.Lock()
	defer m.Unlock()

	if m.shouldRefreshFromBalance {
		m.refreshFromBalance()
	}

	for start := m.highestSuccess + 1; ; start++ {
		if _, ok := m.used[start]; !ok {
			m.used[start] = struct{}{}
			logging.Logger.Info("Next available nonce.", zap.Any("nonce", start))
			return start
		}
	}
}

func (m *nonceMonitor) recordFailedNonce(nonce int64) {
	m.Lock()
	defer m.Unlock()

	delete(m.used, nonce)

	// this is likely a false negative, do nothing else.
	if nonce <= m.highestSuccess {
		return
	}

	m.failed[nonce]++

	// when failing for same nonce often, should reschedule nonce for refresh from balance.
	if m.failed[nonce]%10 == 0 {
		m.shouldRefreshFromBalance = true
	}
}

func (m *nonceMonitor) recordSuccess(nonce int64) {
	m.Lock()
	defer m.Unlock()

	delete(m.used, nonce)

	// if nonce is lower than recorded highest, do nothing.
	// this may be from verification that was late.
	if m.highestSuccess >= nonce {
		return
	}

	m.highestSuccess = nonce

	// (clean up) delete entries on failed up to this new highest success
	for i := m.highestSuccess; i <= nonce; i++ {
		delete(m.failed, i)
	}
}

func (m *nonceMonitor) refreshFromBalance() {
	logging.Logger.Info("Refreshing nonce from balance.")

	// sync lock not necessary, this is expected to be called within a synchronized function.
	m.shouldRefreshFromBalance = false

	cb := &getNonceCallBack{waitCh: make(chan struct{})}
	if err := zcncore.GetNonce(cb); err != nil {
		return
	}

	<-cb.waitCh

	if cb.hasError {
		return
	}

	logging.Logger.Info("Got nonce from balance.", zap.Any("nonce", cb.nonce))

	newNonce := cb.nonce

	oldHighest := m.highestSuccess
	m.highestSuccess = newNonce

	// no clean up necessary if failed is empty
	if len(m.failed) == 0 {
		return
	}

	// (clean up) delete entries on failed up to this new highest success
	for i := oldHighest; i <= m.highestSuccess; i++ {
		delete(m.failed, i)
	}
}

type getNonceCallBack struct {
	waitCh   chan struct{}
	nonce    int64
	hasError bool
}

func (g *getNonceCallBack) OnNonceAvailable(status int, nonce int64, info string) {
	if status != zcncore.StatusSuccess {
		g.hasError = true
	}

	g.nonce = nonce

	close(g.waitCh)
}
