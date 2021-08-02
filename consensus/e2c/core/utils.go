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
	block *types.Block
	time  time.Time
}

type blockQueue struct {
	queue        map[common.Hash]*proposal
	requestQueue map[common.Hash]struct{}
	unhandled    map[common.Hash]*types.Block
	parent       map[common.Hash]*types.Block
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
		parent:       make(map[common.Hash]*types.Block),
		delta:        delta,
		timer:        time.NewTimer(time.Millisecond),
		size:         0,
	}
	return bq
}

func (bq *blockQueue) insertRequest(hash common.Hash) {
	bq.requestQueue[hash] = struct{}{}
}

func (bq *blockQueue) deleteRequest(hash common.Hash) {
	delete(bq.requestQueue, hash)
}

func (bq *blockQueue) deleteUnhandled(hash common.Hash) {
	delete(bq.unhandled, hash)
}

func (bq *blockQueue) insertUnhandled(block *types.Block) {
	bq.unhandled[block.Hash()] = block
	bq.parent[block.ParentHash()] = block
}

func (bq *blockQueue) insertHandled(block *types.Block) {
	delete(bq.unhandled, block.Hash())
	delete(bq.requestQueue, block.Hash())
	if bq.size == 0 {
		bq.timer.Reset(2 * bq.delta * time.Millisecond)
		bq.nextBlock = block.Hash()
	}
	bq.queue[block.Hash()] = &proposal{
		block: block,
		time:  time.Now(),
	}
	bq.size++
}

func (bq *blockQueue) get(hash common.Hash) (*types.Block, bool) {
	p, ok := bq.queue[hash]
	if !ok {
		return nil, ok
	}
	return p.block, ok
}

func (bq *blockQueue) contains(hash common.Hash) bool {
	_, ok := bq.queue[hash]
	_, unhandledOk := bq.unhandled[hash]
	return ok || unhandledOk
}

func (bq *blockQueue) hasRequest(hash common.Hash) bool {
	_, ok := bq.requestQueue[hash]
	return ok
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

func (bq *blockQueue) getNext() (*types.Block, bool) {
	if bq.size == 0 {
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

func (bq *blockQueue) getChild(hash common.Hash) (*types.Block, bool) {
	child, ok := bq.parent[hash]
	delete(bq.parent, hash)
	return child, ok
}
