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

package core

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

// New creates an Istanbul consensus core
func New(backend e2c.Backend, config *e2c.Config) e2c.Engine {
	c := &core{
		config:    config,
		handlerWg: new(sync.WaitGroup),
		logger:    log.New("address", backend.Address()),
		backend:   backend,
		queuedBlocks: make(map[common.Hash]struct {
			block *types.Block
			time  time.Time
		}),
		expectedHeight: big.NewInt(1), // TODO: Make this look for the block at the top of the chain!!!!
		delta:          time.Duration(1000),
	}

	return c
}

// ----------------------------------------------------------------------------

type core struct {
	config        *e2c.Config
	logger        log.Logger
	commitTimer   *time.Timer // alert when a blocks timer expires
	progressTimer *e2c.ProgressTimer
	nextBlock     common.Hash              // Next block whose timer will go off
	queuedBlocks  map[common.Hash]struct { // this is the queue of all blocks not yet committed
		block *types.Block
		time  time.Time
	}

	expectedHeight *big.Int
	backend        e2c.Backend
	eventMux       *event.TypeMuxSubscription

	handlerWg *sync.WaitGroup
	delta     time.Duration
}

func (c *core) Start() error {
	c.commitTimer = time.NewTimer(time.Millisecond)
	c.progressTimer = e2c.NewProgressTimer(4 * c.delta * time.Millisecond)
	c.subscribeEvents()
	go c.loop()
	return nil
}

func (c *core) Stop() error {
	c.unsubscribeEvents()
	c.handlerWg.Wait()
	return nil
}

func (c *core) subscribeEvents() {
	c.eventMux = c.backend.EventMux().Subscribe(
		e2c.NewBlockEvent{},
		e2c.RelayBlockEvent{},
		e2c.BlameEvent{},
	)
}

func (c *core) unsubscribeEvents() {
	c.eventMux.Unsubscribe()
}

func (c *core) resetTimer() error {
	earliestTime := time.Now()
	var earliestBlock common.Hash

	for block, t := range c.queuedBlocks {
		if t.time.Before(earliestTime) {
			earliestTime = t.time
			earliestBlock = block
		}
	}

	d := time.Until(earliestTime.Add(2 * c.delta * time.Millisecond))

	c.commitTimer.Reset(d)
	c.nextBlock = earliestBlock

	return nil
}

func (c *core) verify(block *types.Block) error {

	if err := c.backend.Verify(block); err != nil {
		if err != consensus.ErrUnknownAncestor {
			return err
		}

		parent, exists := c.queuedBlocks[block.ParentHash()]
		if !exists {
			fmt.Println("Blocks Arrived Out of Order")
			return nil // TODO: Return Error
		}
		// check this again, as it hasn't been checked due to there not being a parent
		if parent.block.Number().Uint64()+1 != block.Number().Uint64() {
			return err
		}
		// All ok, check equivocation
	}
	if block.Number().Uint64() != c.expectedHeight.Uint64() {
		return errors.New("Already received block at this height")
	}
	return nil
}

func (c *core) delete(block common.Hash) {
	delete(c.queuedBlocks, block)
}
