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
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

// @todo bug where we commit a block and delete from queue, then receive the next block before fetcher has had time to add to chain
// solution: delete block when new head event is triggered?

// New creates an E2C consensus core
func New(backend e2c.Backend, config *e2c.Config) e2c.Engine {
	c := &core{
		config:    config,
		handlerWg: new(sync.WaitGroup),
		logger:    log.New(),
		backend:   backend,
		//queuedBlocks:   make(map[common.Hash]*proposal),
		blockQueue:     NewBlockQueue(config.Delta),
		expectedHeight: big.NewInt(0),
		blame:          make(map[common.Address]struct{}),
	}

	return c
}

// ----------------------------------------------------------------------------

type core struct {
	config        *e2c.Config
	logger        log.Logger
	progressTimer *e2c.ProgressTimer

	blockQueue *blockQueue
	//queuedBlocks map[common.Hash]*proposal
	blame map[common.Address]struct{}

	expectedHeight *big.Int
	backend        e2c.Backend
	eventMux       *event.TypeMuxSubscription

	handlerWg  *sync.WaitGroup
	lock       *types.Block
	viewChange uint32
}

func (c *core) Start(block *types.Block) error {
	c.lock = block
	c.progressTimer = e2c.NewProgressTimer(4 * c.config.Delta * time.Millisecond)
	c.subscribeEvents()
	atomic.StoreUint32(&c.viewChange, 0)
	go c.loop()
	return nil
}

func (c *core) Stop() error {
	c.unsubscribeEvents()
	c.handlerWg.Wait()
	return nil
}

func (c *core) GetQueuedBlock(hash common.Hash) (*types.Header, error) {
	b, ok := c.blockQueue.get(hash)
	if ok {
		return b.Header(), nil
	}
	if b, ok = c.blockQueue.unhandled[hash]; ok {
		return b.Header(), nil
	}
	return nil, errors.New("unknown block")
}

func (c *core) Lock() *types.Block {
	return c.lock
}

func (c *core) subscribeEvents() {
	c.eventMux = c.backend.EventMux().Subscribe(
		e2c.NewBlockEvent{},
		e2c.RelayBlockEvent{},
		e2c.BlameEvent{},
		e2c.BlameCertificate{},
		e2c.RequestBlockEvent{},
		e2c.RespondToRequestEvent{},
	)
}

func (c *core) unsubscribeEvents() {
	c.eventMux.Unsubscribe()
}

func (c *core) verify(block *types.Block) error {
	if err := c.backend.Verify(block); err != nil {
		return err
	}
	if block.Number().Uint64() != (c.lock.Number().Uint64() + 1) {
		//@todo add real error here
		fmt.Println("Number:", block.Number(), "Expected:", c.lock.Number().Uint64()+1)
		return errors.New("equivocation detected")
	}
	return nil
}

func (c *core) commit(block *types.Block) {

	// we are changing view, do not commit any blocks!
	if atomic.LoadUint32(&c.viewChange) == 1 {
		return
	}
	c.backend.Commit(block)
	c.logger.Info("Successfully committed block", "number", block.Number().Uint64(), "txs", len(block.Transactions()), "hash", block.Hash())
}

// @todo add a timeout feature!
func (c *core) requestBlock(hash common.Hash, addr common.Address) {
	c.blockQueue.insertRequest(hash)
	go c.backend.RequestBlock(hash, addr)
}

func (c *core) sendBlame() {
	c.blame[c.backend.Address()] = struct{}{}
	// @todo race condition here
	time.AfterFunc(2*c.config.Delta*time.Millisecond, func() {
		delete(c.blame, c.backend.Address())
	})
	c.backend.SendBlame()
}
