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
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/core/types"
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
				c.handleBlock(ev.Block)
			case e2c.RelayBlockEvent:
				c.handleRelay(ev.Hash, ev.Address)
			case e2c.BlameEvent:
				c.handleBlame(ev.Time)
			case e2c.RequestBlockEvent:
				c.handleRequest(ev.Hash, ev.Address)
			case e2c.RespondToRequestEvent:
				c.handleResponse(ev.Block)
			}

		case <-c.progressTimer.Chan():
			// c.backend.SendBlame(common.Hash{})
			c.logger.Info("Progress Timer expired! Sending Blame message!")
		}
	}
}

func (c *core) handleCommit(block *types.Block) {
	c.backend.Commit(block)
	delete(c.queuedBlocks, block.Hash())
	c.logger.Info("Successfully committed block", "number", block.Number().Uint64(), "txs", len(block.Transactions()), "hash", block.Hash())
}

func (c *core) handleBlock(block *types.Block) error {

	if err := c.verify(block); err != nil {
		if err == consensus.ErrUnknownAncestor {
			// @todo if expected block is n and what we got is k > n+1, we request 1 at a time. Fix this to request all at once
			c.requestBlock(block.ParentHash(), common.Address{})
			c.unhandledBlocks[block.Hash()] = block
			c.logger.Debug("Requesting missing block", "hash", block.Hash())
			return nil
		} else {
			// c.backend.SendBlame()
			return err
		}
	} // @todo handle potential errors from this

	c.logger.Info("Valid block received", "number", block.Number().Uint64(), "hash", block.Hash())
	c.progressTimer.AddDuration(2 * c.delta * time.Millisecond)
	c.backend.RelayBlock(block.Hash())

	c.queuedBlocks[block.Hash()] = block
	go time.AfterFunc(2*c.delta*time.Millisecond, func() {
		c.handleCommit(block)
	})

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
	if _, ok := c.unhandledBlocks[hash]; ok {
		return nil
	}

	c.requestBlock(hash, addr)
	return nil
}

func (c *core) handleBlame(t time.Time) error {

	/*
		fmt.Println("Blame Received!")
		if _, ok := c.blamedBlocks[hash]; !ok {
			c.blamedBlocks[hash] = 1
		} else {
			c.blamedBlocks[hash]++
		}

		if c.blamedBlocks[hash] > 1 {
			// trigger view change!
			c.backend.ChangeView()
		}
	*/
	return nil
}

func (c *core) handleRequest(hash common.Hash, addr common.Address) error {

	c.logger.Debug("Request Received", "hash", hash, "address", addr)
	block, err := c.backend.GetBlockFromChain(hash)
	if err != nil {
		b, ok := c.queuedBlocks[hash]
		if !ok {
			c.logger.Debug("Don't have requested block", "hash", hash, "address", addr)
			return errors.New("don't have requested block")
		}
		block = b
	}

	go c.backend.RespondToRequest(block, addr)
	return nil
}

func (c *core) handleResponse(block *types.Block) error {

	if _, ok := c.requestedBlocks[block.Hash()]; !ok {
		c.logger.Debug("Received response for unrequested block", "number", block.Number().Uint64(), "hash", block.Hash())
		return errors.New("didn't request block")
	}

	c.logger.Debug("Response to request received", "number", block.Number().Uint64(), "hash", block.Hash())
	delete(c.requestedBlocks, block.Hash())
	if err := c.handleBlock(block); err != nil {
		return err
	}

	for _, unhandledBlock := range c.unhandledBlocks {
		if unhandledBlock.ParentHash() == block.Hash() {
			c.handleBlock(unhandledBlock)
			delete(c.unhandledBlocks, unhandledBlock.Hash())
			block = unhandledBlock
		}
	}
	return nil
}
