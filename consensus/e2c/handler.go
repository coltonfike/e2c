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
	"github.com/ethereum/go-ethereum/event"
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
	eventMux       *event.TypeMuxSubscription
	handlerwg      *sync.WaitGroup
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
		handlerwg:      new(sync.WaitGroup),
	}
}

func (h *E2CHandler) Start() {
	h.commitTimer = time.NewTimer(time.Millisecond)
	h.subscribeEvents()
	go h.loop()
}

func (h *E2CHandler) subscribeEvents() {
	h.eventMux = h.e2c.eventMux.Subscribe(BlockProposal{})
}

func (h *E2CHandler) unsubscribeEvents() {
	h.eventMux.Unsubscribe()
}

func (h *E2CHandler) Stop() error {
	h.unsubscribeEvents()
	h.handlerwg.Wait()
	return nil
}

func (h *E2CHandler) loop() {
	h.handlerwg.Add(1)
	for {
		select {
		case event, ok := <-h.eventMux.Chan():
			if !ok {
				return
			}
			switch ev := event.Data.(type) {
			case BlockProposal:
				h.handleBlock(ev.id, ev.block)
			}
		case <-h.commitTimer.C:

			// No blocks to work on
			if len(h.queuedBlocks) == 0 {
				h.commitTimer.Reset(time.Millisecond)
				continue
			}

			fmt.Println("Timer Expired, beinging commits")
			h.commit(h.nextBlock)
			fmt.Println("Commits Finished")
			h.resetTimer()
		}
	}
}

func (h *E2CHandler) commit(block common.Hash) {

	// commit the current block
	h.e2c.broadcaster.Enqueue(h.queuedBlocks[block].id, h.queuedBlocks[block].block)
	fmt.Println("Committed block: " + h.queuedBlocks[block].block.Number().String() + " at time: " + time.Now().String())
	delete(h.queuedBlocks, block)

	// commit all the ancestors
	for {
		if ancestor, exists := h.ancestorBlocks[block]; exists {
			h.e2c.broadcaster.Enqueue(h.queuedBlocks[ancestor].id, h.queuedBlocks[ancestor].block)
			fmt.Println("Committed block: " + h.queuedBlocks[ancestor].block.Number().String() + " because it was ancestor to previously committed block. at time: " + time.Now().String())
			delete(h.ancestorBlocks, block)
			delete(h.queuedBlocks, ancestor)
			block = ancestor
		} else {
			return
		}
	}
}

func (h *E2CHandler) handleBlock(id string, block *types.Block) {
	fmt.Println("Block " + block.Number().String() + " received by handler")
	if len(h.queuedBlocks) == 0 {
		fmt.Println("Queue empty for block " + block.Number().String() + ". Resetting timer")
		h.commitTimer.Reset(time.Duration(h.e2c.delta) * time.Second)
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
	fmt.Println("Block " + block.Number().String())
}

func (h *E2CHandler) resetTimer() {
	earliestTime := time.Now()
	var earliestBlock common.Hash

	for block, t := range h.queuedBlocks {
		if t.time.Before(earliestTime) {
			earliestTime = t.time
			earliestBlock = block
		}
	}

	fmt.Println("Timer is set to trigger in " + time.Until(earliestTime.Add(time.Duration(h.e2c.delta)*time.Second)).String())
	h.commitTimer.Reset(time.Until(earliestTime.Add(time.Duration(h.e2c.delta) * time.Second)))
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

func (e2c *E2C) HandleNewBlock(block *types.Block, id string, mark func(common.Hash), hash common.Hash) (bool, error) {

	fmt.Println("Block " + block.Number().String() + " Received")
	go e2c.broadcaster.BroadcastBlock(block, false)
	fmt.Println("Block " + block.Number().String() + " relayed")

	e2c.eventMux.Post(BlockProposal{
		id:    id,
		block: block,
	})
	fmt.Println("Block " + block.Number().String() + " sent to handler")
	mark(hash)

	return false, nil
}
