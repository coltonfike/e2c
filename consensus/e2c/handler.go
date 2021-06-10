// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package e2c

import (
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/p2p"
)

type E2CHandler struct {
	e2c          *E2C
	commitTimer  *time.Timer              // alert when a blocks timer expires
	nextBlock    common.Hash              // Next block whose timer will go off
	queuedBlocks map[common.Hash]struct { // this is the queue of all blocks not yet committed
		id    string
		block *types.Block
		time  time.Time
	}
	ancestorBlocks map[common.Hash]common.Hash // Allows for tracking ancestors
	mux            *sync.Mutex                 // lock for race conditions
}

func NewE2CHandler(e2c *E2C) *E2CHandler {
	return &E2CHandler{
		e2c: e2c,
		queuedBlocks: make(map[common.Hash]struct {
			id    string
			block *types.Block
			time  time.Time
		}),
		ancestorBlocks: make(map[common.Hash]common.Hash),
		mux:            new(sync.Mutex),
	}
}

func (h *E2CHandler) Start() {
	h.commitTimer = time.NewTimer(time.Millisecond)
	go h.loop()
}

func (h *E2CHandler) loop() {
	for {
		select {
		case <-h.commitTimer.C:

			// No blocks to work on
			if len(h.queuedBlocks) == 0 {
				h.resetTimer()
				continue
			}

			h.commitAncestors(h.nextBlock)
			h.resetTimer()
		}
	}
}

func (h *E2CHandler) commitAncestors(block common.Hash) {
	h.mux.Lock()
	defer h.mux.Unlock()

	h.e2c.broadcaster.Enqueue(h.queuedBlocks[block].id, h.queuedBlocks[block].block)
	fmt.Println("Committed block: " + h.queuedBlocks[block].block.Number().String())
	delete(h.queuedBlocks, block)
	for {
		if ancestor, exists := h.ancestorBlocks[block]; exists {
			h.e2c.broadcaster.Enqueue(h.queuedBlocks[ancestor].id, h.queuedBlocks[ancestor].block)
			fmt.Println("Committed block: " + h.queuedBlocks[ancestor].block.Number().String())
			delete(h.ancestorBlocks, block)
			delete(h.queuedBlocks, ancestor)
			block = ancestor
		} else {
			return
		}
	}
}

func (h *E2CHandler) HandleBlock(id string, block *types.Block) {
	h.mux.Lock()
	defer h.mux.Unlock()

	if len(h.queuedBlocks) == 0 {
		h.commitTimer.Reset(10 * time.Second)
		h.nextBlock = block.Hash()
	}

	h.queuedBlocks[block.Hash()] = struct {
		id    string
		block *types.Block
		time  time.Time
	}{
		id:    id,
		block: block,
		time:  time.Now(),
	}
	h.ancestorBlocks[block.ParentHash()] = block.Hash()
}

func (h *E2CHandler) resetTimer() {
	h.mux.Lock()
	defer h.mux.Unlock()

	if len(h.queuedBlocks) == 0 {
		h.commitTimer.Reset(time.Millisecond)
		return
	}

	earliestTime := time.Now()
	var earliestBlock common.Hash

	for block, t := range h.queuedBlocks {
		if t.time.Before(earliestTime) {
			earliestTime = t.time
			earliestBlock = block
		}
	}

	h.commitTimer.Reset(time.Until(earliestTime.Add(time.Second)))
	h.nextBlock = earliestBlock
}

// HandleMsg implements consensus.Handler.HandleMsg
func (e2c *E2C) HandleMsg(addr common.Address, msg p2p.Msg) (bool, error) {
	return false, nil
}

// SetBroadcaster implements consensus.Handler.SetBroadcaster
func (e2c *E2C) SetBroadcaster(broadcaster consensus.Broadcaster) {
	e2c.broadcaster = broadcaster
}

func (e2c *E2C) NewChainHead() error {
	return nil
}

// TODO: Commit all known ancestors of the block
func (e2c *E2C) HandleNewBlock(block *types.Block, id string, mark func(common.Hash), hash common.Hash) (bool, error) {

	fmt.Println("Block " + block.Number().String() + " Received! Broadcasting to peers...")
	go e2c.broadcaster.BroadcastBlock(block, false)

	// TODO: add event queue for this, it's inefficient as it currently is due to this method using locks
	// If the handler is doing something else (like committing all ancestors) this hangs until that's done
	// I'll add an event queue for this so the program doesn't hang here
	e2c.handler.HandleBlock(id, block)
	mark(hash)
	//e2c.broadcaster.Enqueue(id, block)

	return false, nil
}
