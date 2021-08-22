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
	"math/big"
	"sync"
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
func New(backend e2c.Backend, config *e2c.Config, ch chan *types.Block) e2c.Engine {
	c := &core{
		config:         config,
		handlerWg:      new(sync.WaitGroup),
		logger:         log.New(),
		backend:        backend,
		blockQueue:     NewBlockQueue(config.Delta),
		expectedHeight: big.NewInt(0),
		blockCh:        ch,
		blame:          make(map[common.Address]*Message),
		validates:      make(map[common.Address]*Message),
		votes:          make(map[common.Hash]map[common.Address]*Message),
	}

	return c
}

// ----------------------------------------------------------------------------

type core struct {
	config        *e2c.Config
	logger        log.Logger
	blockCh       chan *types.Block
	progressTimer *ProgressTimer
	certTimer     *time.Timer

	blockQueue *blockQueue
	blame      map[common.Address]*Message
	validates  map[common.Address]*Message
	votes      map[common.Hash]map[common.Address]*Message

	expectedHeight *big.Int
	backend        e2c.Backend
	eventMux       *event.TypeMuxSubscription

	handlerWg   *sync.WaitGroup
	lock        *types.Block
	committed   *types.Block
	highestCert *BlockCertificate
}

func (c *core) Start(block *types.Block) error {
	c.lock = block
	c.progressTimer = NewProgressTimer(c.config.Delta * time.Millisecond)
	c.certTimer = time.NewTimer(1 * time.Millisecond)
	c.subscribeEvents()
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
		e2c.MessageEvent{},
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
		return errors.New("equivocation detected")
	}
	return nil
}

func (c *core) commit(block *types.Block) {

	c.backend.Commit(block)
	c.committed = block
	c.logger.Info("[E2C] Successfully committed block", "number", block.Number().Uint64(), "txs", len(block.Transactions()), "hash", block.Hash())
}

func (c *core) finalizeMessage(msg *Message) ([]byte, error) {
	msg.Address = c.backend.Address()
	msg.View = c.backend.View()

	data, err := msg.PayloadWithSig(c.backend.Sign)
	if err != nil {
		return nil, err
	}

	return data, nil
}

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

func (c *core) checkValidatorSignature(data []byte, sig []byte) (common.Address, error) {
	return e2c.CheckValidatorSignature(c.backend.Validators(), data, sig)
}
