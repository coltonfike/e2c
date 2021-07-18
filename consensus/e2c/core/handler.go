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
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/e2c"
)

func (c *core) loop() {
	defer func() {
		c.handlerWg.Done()
	}()

	c.handlerWg.Add(1)

	for {
		select {
		case event, ok := <-c.eventMux.Chan():
			if !ok {
				return
			}

			switch ev := event.Data.(type) {
			case e2c.NewBlockEvent:
				if _, ok := c.queuedBlocks[ev.Block.Hash()]; !ok {
					c.queuedBlocks[ev.Block.Hash()] = &proposal{
						block: ev.Block,
					}
				}
				c.handleBlock(c.queuedBlocks[ev.Block.Hash()])
			case e2c.RelayBlockEvent:
				c.handleRelay(ev.Hash, ev.Address)
			case e2c.BlameEvent:
				c.handleBlame(ev.Time, ev.Address)
			case e2c.RequestBlockEvent:
				c.handleRequest(ev.Hash, ev.Address)
			case e2c.RespondToRequestEvent:
				p := c.queuedBlocks[ev.Block.Hash()]
				p.block = ev.Block
				c.handleResponse(p)
			}

		case <-c.progressTimer.Chan():
			if c.backend.Address() != c.backend.Leader() {
				c.sendBlame()
				c.logger.Info("Progress Timer expired! Sending Blame message!")
			}
		}
	}
}

func (c *core) handleBlock(p *proposal) error {

	if err := c.verify(p.block); err != nil {
		if err == consensus.ErrUnknownAncestor {

			// @todo if expected block is n and what we got is k > n+1, we request 1 at a time. Fix this to request all at once

			p.status = UNHANDLED
			c.requestBlock(p.block.ParentHash(), common.Address{})
			c.logger.Debug("Requesting missing block", "hash", p.block.Hash())
			return nil
		} else {
			c.sendBlame()
			return err
		}
	}

	c.logger.Info("Valid block received", "number", p.block.Number().Uint64(), "hash", p.block.Hash())
	c.progressTimer.AddDuration(2 * c.delta * time.Millisecond)
	c.backend.RelayBlock(p.block.Hash())

	time.AfterFunc(2*c.delta*time.Millisecond, func() {
		c.commit(p.block)
	})
	p.status = HANDLED

	c.expectedHeight.Add(c.expectedHeight, big.NewInt(1))
	return nil
}

func (c *core) handleRelay(hash common.Hash, addr common.Address) error {

	// if any of there are true, we have the block
	c.logger.Debug("Relay received", "hash", hash, "address", addr)
	if _, err := c.backend.GetBlockFromChain(hash); err == nil {
		return nil
	}
	if _, ok := c.queuedBlocks[hash]; ok {
		return nil
	}

	c.requestBlock(hash, addr)
	return nil
}

func (c *core) handleBlame(t time.Time, addr common.Address) error {

	if _, ok := c.blame[addr]; !ok {
		c.blame[addr] = struct{}{}
	}

	c.logger.Info("Blame message received", "total blame", len(c.blame))
	if len(c.blame) > 1 {
		atomic.StoreUint32(&c.viewChange, 1)
		c.backend.ChangeView()
	}
	return nil
}

func (c *core) handleRequest(hash common.Hash, addr common.Address) error {

	c.logger.Debug("Request Received", "hash", hash, "address", addr)
	block, err := c.backend.GetBlockFromChain(hash)
	if err != nil {
		p, ok := c.queuedBlocks[hash]
		if !ok || p.status != HANDLED {
			c.logger.Debug("Don't have requested block", "hash", hash, "address", addr)
			return errors.New("don't have requested block")
		}
		block = p.block
	}

	go c.backend.RespondToRequest(block, addr)
	return nil
}

func (c *core) handleResponse(p *proposal) error {

	c.logger.Debug("Response to request received", "number", p.block.Number().Uint64(), "hash", p.block.Hash())

	p.status = UNHANDLED
	if err := c.handleBlock(p); err != nil {
		return err
	}

	for _, unhandled := range c.queuedBlocks {
		if unhandled.status == UNHANDLED && unhandled.block.ParentHash() == p.block.Hash() {
			c.handleBlock(unhandled)
			p = unhandled
		}
	}
	return nil
}
