package core

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const (
	HANDLED   = 0
	UNHANDLED = 1
	REQUESTED = 2
)

type proposal struct {
	block  *types.Block
	status uint8
	time   time.Time
}

type blockQueue struct {
	queue        map[common.Hash]*proposal
	requestQueue map[common.Hash]struct{}
	unhandled    map[common.Hash]*types.Block
	nextBlock    common.Hash
	lastBlock    common.Hash
	timer        *time.Timer
	delta        time.Duration
	size         uint64 // we have to track this locally
}

func NewBlockQueue(delta time.Duration) *blockQueue {
	bq := &blockQueue{
		queue:        make(map[common.Hash]*proposal),
		requestQueue: make(map[common.Hash]struct{}),
		unhandled:    make(map[common.Hash]*types.Block),
		delta:        delta,
		timer:        time.NewTimer(time.Millisecond),
		size:         0,
	}
	return bq
}

func (bq *blockQueue) addRequest(hash common.Hash) {
	bq.requestQueue[hash] = struct{}{}
}

func (bq *blockQueue) deleteRequest(hash common.Hash) {
	delete(bq.requestQueue, hash)
}

func (bq *blockQueue) deleteUnhandled(hash common.Hash) {
	delete(bq.unhandled, hash)
}

func (bq *blockQueue) addUnhandled(block *types.Block) {
	bq.unhandled[block.Hash()] = block
}

func (bq *blockQueue) insert(p *proposal) {
	if bq.size == 0 {
		bq.timer.Reset(2 * bq.delta * time.Millisecond)
		bq.nextBlock = p.block.Hash()
	}
	bq.queue[p.block.Hash()] = p
	bq.size++
}

func (bq *blockQueue) get(hash common.Hash) (*proposal, bool) {
	p, ok := bq.queue[hash]
	return p, ok
}

func (bq *blockQueue) delete(hash common.Hash) {
	delete(bq.queue, hash)
}

func (bq *blockQueue) resetTimer() {
	earliestTime := time.Now()
	var earliestBlock common.Hash

	for block, p := range bq.queue {
		if block != bq.lastBlock && p.time.Before(earliestTime) {
			earliestTime = p.time
			earliestBlock = block
		}
	}

	d := time.Until(earliestTime.Add(2 * bq.delta * time.Millisecond))

	bq.timer.Reset(d)
	bq.nextBlock = earliestBlock
}

func (bq *blockQueue) c() <-chan time.Time {
	return bq.timer.C
}

func (bq *blockQueue) getNext() (*proposal, bool) {
	if len(bq.queue) == 0 {
		bq.timer.Reset(time.Millisecond)
		return nil, false
	}
	bq.delete(bq.lastBlock)
	p, _ := bq.get(bq.nextBlock)
	bq.lastBlock = bq.nextBlock
	bq.resetTimer()
	bq.size--
	return p, true
}
