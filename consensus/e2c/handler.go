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
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/trie"
)

const (
	NewBlockMsg = 0x07
	AckMsg      = 0x0c
)

type E2CHandler struct {
	e2c           *E2C
	commitTimer   *time.Timer // alert when a blocks timer expires
	progressTimer *ProgressTimer
	nextBlock     common.Hash              // Next block whose timer will go off
	queuedBlocks  map[common.Hash]struct { // this is the queue of all blocks not yet committed
		block *types.Block
		time  time.Time
	}
	eventMux  *event.TypeMuxSubscription
	handlerwg *sync.WaitGroup
}

func NewE2CHandler(e2c *E2C) *E2CHandler {
	return &E2CHandler{
		e2c: e2c,
		queuedBlocks: make(map[common.Hash]struct {
			block *types.Block
			time  time.Time
		}),
		handlerwg: new(sync.WaitGroup),
	}
}

func (h *E2CHandler) Start() {
	h.commitTimer = time.NewTimer(time.Millisecond)
	h.progressTimer = NewProgressTimer(4 * time.Duration(h.e2c.delta) * time.Second)
	h.subscribeEvents()
	go h.loop()
}

func (h *E2CHandler) subscribeEvents() {
	h.eventMux = h.e2c.eventMux.Subscribe(BlockProposal{}, Ack{})
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
				if err := h.handleBlock(ev.block); err != nil {
					fmt.Println("Problem handling block:", err)
				}
			}

		case <-h.commitTimer.C:
			// No blocks to work on
			if len(h.queuedBlocks) == 0 {
				h.commitTimer.Reset(time.Millisecond)
				continue
			}

			if err := h.commit(h.nextBlock); err != nil {
				fmt.Println("Problem handling commit:", err)
			}
			if err := h.resetTimer(); err != nil {
				fmt.Println("Problem handling timer:", err)
			}

		case <-h.progressTimer.timer.C:
			fmt.Println("Progress Timer expired! Sending Blame message!")
		}
	}
}

func (h *E2CHandler) commit(block common.Hash) error {

	if _, err := h.e2c.broadcaster.InsertBlock(h.queuedBlocks[block].block); err != nil {
		h.delete(block)
		return err
	}
	fmt.Println("Successfully committed block", h.queuedBlocks[block].block.Number().String())
	h.delete(block)
	return nil
}

func (h *E2CHandler) handleBlock(block *types.Block) error {

	// TODO: These ifs are not doing what they should be. I'll need to write a
	// better method for checking if the block is completely valid and just not yet
	// put on the chain
	if err := h.e2c.broadcaster.VerifyHeader(block.Header()); err != nil {
		if _, exists := h.queuedBlocks[block.ParentHash()]; !exists {
			if block.Number().String() != "1" {
				return errors.New("Invalid block!")
			}
		}
	}

	fmt.Println("Valid block", block.Number().String(), "received!")
	h.progressTimer.AddDuration(2 * time.Duration(h.e2c.delta) * time.Second)
	go h.e2c.broadcaster.BroadcastBlock(block, false)

	if len(h.queuedBlocks) == 0 {
		h.commitTimer.Reset(2 * time.Duration(h.e2c.delta) * time.Second)
		h.nextBlock = block.Hash()
	}

	h.queuedBlocks[block.Hash()] = struct {
		block *types.Block
		time  time.Time
	}{
		block: block,
		time:  time.Now(),
	}
	return nil
}

func (h *E2CHandler) resetTimer() error {
	earliestTime := time.Now()
	var earliestBlock common.Hash

	for block, t := range h.queuedBlocks {
		if t.time.Before(earliestTime) {
			earliestTime = t.time
			earliestBlock = block
		}
	}

	d := time.Until(earliestTime.Add(2 * time.Duration(h.e2c.delta) * time.Second))
	if d <= 0 {
		return errors.New("Timer already expired")
	}

	h.commitTimer.Reset(d)
	h.nextBlock = earliestBlock

	return nil
}

func (h *E2CHandler) delete(block common.Hash) {
	delete(h.queuedBlocks, block)
}

func (e2c *E2C) HandleMsg(p consensus.Peer, msg p2p.Msg) (bool, error) {
	if msg.Code == NewBlockMsg {
		var request struct {
			Block *types.Block
			TD    *big.Int
		}

		// All error checking, if any of these fail, don't handleMsg and let the caller deal with it
		// Most of these require some special handling that can't be done here due to due to circular
		// imports, so I just return this to caller so the caller can deal with. Not the most efficient
		// solution, but it should work for the moment. This is a section that could slightly improve
		// efficiency if needed later on
		if err := msg.Decode(&request); err != nil {
			return false, nil
		}
		if hash := types.DeriveSha(request.Block.Transactions(), new(trie.Trie)); hash != request.Block.TxHash() {
			return false, nil
		}
		if err := request.Block.SanityCheck(); err != nil {
			return false, nil
		}

		request.Block.ReceivedAt = msg.ReceivedAt
		request.Block.ReceivedFrom = p

		p.MarkBlock(request.Block.Hash())

		e2c.eventMux.Post(BlockProposal{
			block: request.Block,
		})

		return true, nil

	} else if msg.Code == AckMsg {

		var hash common.Hash
		if err := msg.Decode(&hash); err != nil {
			return false, nil
		}

		e2c.eventMux.Post(Ack{
			id:    p.String(),
			block: hash,
		})

		return true, nil
	}
	return false, nil
}

// SetBroadcaster implements consensus.Handler.SetBroadcaster
func (e2c *E2C) SetBroadcaster(broadcaster consensus.Broadcaster) {
	e2c.broadcaster = broadcaster
}

// TODO: Figure out what this is
func (e2c *E2C) NewChainHead() error {
	return nil
}
