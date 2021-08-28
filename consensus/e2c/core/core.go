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
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

// New creates an E2C consensus core
func New(backend e2c.Backend, config *e2c.Config, ch chan *types.Block) e2c.Engine {
	c := &core{
		config:     config,
		handlerWg:  new(sync.WaitGroup),
		logger:     log.New(),
		backend:    backend,
		blockQueue: NewBlockQueue(config.Delta),
		blockCh:    ch,
		blame:      make(map[common.Address][]byte),
		validates:  make(map[common.Address][]byte),
		votes:      make(map[common.Hash]map[common.Address][]byte),
	}

	return c
}

// ----------------------------------------------------------------------------

type core struct {
	config  *e2c.Config
	logger  log.Logger
	blockCh chan *types.Block

	progressTimer *ProgressTimer // tracks the progress of leader
	certTimer     *time.Timer    // this is only used in view change to wait the 4 delta

	// Data structures for core
	blockQueue *blockQueue
	blame      map[common.Address][]byte
	validates  map[common.Address][]byte
	votes      map[common.Hash]map[common.Address][]byte

	backend   e2c.Backend
	eventMux  *event.TypeMuxSubscription
	handlerWg *sync.WaitGroup

	lock        *types.Block
	committed   *types.Block
	highestCert *BlockCertificate
}

// initializes data
func (c *core) Start(block *types.Block) error {
	c.lock = block
	c.progressTimer = NewProgressTimer(c.config.Delta * time.Millisecond)
	c.certTimer = time.NewTimer(1 * time.Millisecond)
	c.subscribeEvents()

	// start event loop
	go c.loop()
	return nil
}

// clears memory and forces loop to stop
func (c *core) Stop() error {
	c.unsubscribeEvents()
	c.handlerWg.Wait()
	return nil
}

// this will search our data structures for the block. It's called by VerifyHeader in backend
// it shouldn't cause any race conditions as the call for this only exists in an if inside VerifyHeader.
// that if only executes if the caller to VerifyHeader was the core event loop, which means there won't be
// concurrent access
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

// adds the event to the eventmux
func (c *core) subscribeEvents() {
	c.eventMux = c.backend.EventMux().Subscribe(
		e2c.MessageEvent{},
	)
}

// clear events from eventmux
func (c *core) unsubscribeEvents() {
	c.eventMux.Unsubscribe()
}

// check that blocks are valid
func (c *core) verify(block *types.Block) error {
	if err := c.backend.Verify(block); err != nil {
		return err
	}
	if block.Number().Uint64() != (c.lock.Number().Uint64() + 1) {
		//@todo add real error here
		return errors.New("equivocation detected")
	}
	return nil
}

// add the block to the chain
func (c *core) commit(block *types.Block) {

	c.backend.Commit(block)
	c.committed = block
	c.logger.Info("[E2C] Successfully committed block", "number", block.Number().Uint64(), "txs", len(block.Transactions()), "hash", block.Hash())
}

// this signs the message and adds the view and addess to the packet
func (c *core) finalizeMessage(msg *Message) ([]byte, error) {
	msg.Address = c.backend.Address()
	msg.View = c.backend.View()

	data, err := msg.PayloadWithSig(c.backend.Sign)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// sends the message to all nodes
func (c *core) broadcast(msg *Message) {
	payload, err := c.finalizeMessage(msg)
	if err != nil {
		c.logger.Error("Failed to finalize message", "msg", msg, "err", err)
		return
	}

	if err = c.backend.Broadcast(payload); err != nil {
		c.logger.Error("Failed to broadcast message", "msg", msg, "err", err)
		return
	}
}

// sends message to a single node
func (c *core) send(msg *Message, addr common.Address) {
	payload, err := c.finalizeMessage(msg)
	if err != nil {
		c.logger.Error("Failed to finalize message", "msg", msg, "err", err)
		return
	}

	if err = c.backend.Send(payload, addr); err != nil {
		c.logger.Error("Failed to send message", "msg", msg, "err", err, "addr", addr)
		return
	}
}

// this is a helper method that is used by core/messages.go to verify the signature came from a validator
func (c *core) checkValidatorSignature(data []byte, sig []byte) (common.Address, error) {
	return e2c.CheckValidatorSignature(c.backend.Validators(), data, sig)
}
