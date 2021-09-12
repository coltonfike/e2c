package core

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type proposal struct {
	block *types.Block
	time  time.Time
}

// blockqueue allows us to track all the blocks we are currently handling in one place
type blockQueue struct {
	queue        map[common.Hash]*proposal
	requestQueue map[common.Hash]struct{}
	unhandled    map[common.Hash]*types.Block
	parent       map[common.Hash]*types.Block
	byNumber     map[uint64]*types.Block
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
		byNumber:     make(map[uint64]*types.Block),
		delta:        delta,
		timer:        time.NewTimer(time.Millisecond),
		size:         0,
	}
	return bq
}

// adds a hash to our request structure
func (bq *blockQueue) insertRequest(hash common.Hash) {
	bq.requestQueue[hash] = struct{}{}
}

func (bq *blockQueue) deleteRequest(hash common.Hash) {
	delete(bq.requestQueue, hash)
}

func (bq *blockQueue) deleteUnhandled(hash common.Hash) {
	delete(bq.unhandled, hash)
}

// adds a block to our unhandled structure
func (bq *blockQueue) insertUnhandled(block *types.Block) {
	bq.unhandled[block.Hash()] = block
	bq.parent[block.ParentHash()] = block
}

// adds a handled block to the queue
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
	bq.byNumber[block.Number().Uint64()] = block
	bq.size++
}

// retrieves a block from our handled queue
func (bq *blockQueue) get(hash common.Hash) (*types.Block, bool) {
	p, ok := bq.queue[hash]
	if !ok {
		return nil, ok
	}
	return p.block, ok
}

func (bq *blockQueue) getByNumber(num uint64) (*types.Block, bool) {
	block, ok := bq.byNumber[num]
	if !ok {
		return nil, ok
	}
	return block, ok
}

// checks to see if we have the block anywhere, either the handled queue or unhandledqueue
// imagine we don't have block 3 but we do have 4. We can't handle block 4 since we don't have 3 so it's in our unhandled queue
// Now imagine we get block 5. We would fail verification on it since we can't verify it's parent is valid, but we don't need to
// request block 4 because we already have it. this method is used for this exact situation
func (bq *blockQueue) contains(hash common.Hash) bool {
	_, ok := bq.queue[hash]
	_, unhandledOk := bq.unhandled[hash]
	return ok || unhandledOk
}

// checks to see if we have requested the block
func (bq *blockQueue) hasRequest(hash common.Hash) bool {
	_, ok := bq.requestQueue[hash]
	return ok
}

func (bq *blockQueue) delete(hash common.Hash) {
	if b, ok := bq.queue[hash]; ok {
		delete(bq.byNumber, b.block.Number().Uint64())
	}
	delete(bq.queue, hash)
}

// after committing a block, we need to reset the timer to expire at the time the next block is to be committed
func (bq *blockQueue) resetTimer() {
	earliestTime := time.Now()
	var earliestBlock common.Hash

	// find earliestTime in the queue
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

// returns the channel so our event loop can see when a timer has expired
func (bq *blockQueue) c() <-chan time.Time {
	return bq.timer.C
}

// gives the next block in the queue for commit, but also resets the state to get ready for the next block
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

// this gives us the child of the block. Imagine we have block 4 and 5, but not 3.
// when we get block 3, we want to commit 4 and 5 as well, but 3 has no reference to what came after it
// This method allows us to find block 4 given block 3
func (bq *blockQueue) getChild(hash common.Hash) (*types.Block, bool) {
	child, ok := bq.parent[hash]
	delete(bq.parent, hash)
	return child, ok
}

// progress timer allows us to easily keep track of leaders progress
type ProgressTimer struct {
	timer *time.Timer
	end   time.Time
	delta time.Duration
}

func NewProgressTimer(t time.Duration) *ProgressTimer {
	return &ProgressTimer{time.NewTimer(4 * t), time.Now().Add(t), t}
}

// resets the timer
func (pt *ProgressTimer) Reset(t time.Duration) {
	pt.timer.Reset(t * pt.delta)
	pt.end = time.Now().Add(t * pt.delta)
}

// adds the specified duration to the timer
func (pt *ProgressTimer) AddDuration(t time.Duration) {
	d := time.Until(pt.end) + t*pt.delta
	pt.timer.Reset(d)
	pt.end = time.Now().Add(d)
}

func (pt *ProgressTimer) c() <-chan time.Time {
	return pt.timer.C
}
