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
				if err := c.handleBlock(ev.Block); err != nil {
					fmt.Println("Problem handling block:", err)
				}
			case e2c.RelayBlockEvent:
				if err := c.handleRelay(ev.Hash, ev.Address); err != nil {
					fmt.Println("Problem handling relay:", err)
				}
			case e2c.BlameEvent:
				if err := c.handleBlame(ev.Address); err != nil {
					fmt.Println("Problem handling blame:", err)
				}
			case e2c.RequestBlockEvent:
				if err := c.handleRequest(ev.Hash, ev.Address); err != nil {
					fmt.Println("Problem handling request:", err)
				}
			case e2c.RespondToRequestEvent:
				if err := c.handleResponse(ev.Block); err != nil {
					fmt.Println("Problem handling response:", err)
				}
			}

		case <-c.commitTimer.C:
			// No blocks to work on
			if len(c.queuedBlocks) == 0 {
				c.commitTimer.Reset(time.Millisecond)
				continue
			}

			if err := c.handleCommit(c.nextBlock); err != nil {
				fmt.Println("Problem handling commit:", err)
			}
			if err := c.resetTimer(); err != nil {
				fmt.Println("Problem handling timer:", err)
			}

		case <-c.progressTimer.Chan():
			fmt.Println("Progress Timer expired! Sending Blame message!")
		}
	}
}

func (c *core) handleCommit(block common.Hash) error {

	err := c.backend.Commit(c.queuedBlocks[block].block)
	if err == nil {
		fmt.Println("Successfully committed block", c.queuedBlocks[block].block.Number().String())
	}
	c.delete(block)
	return err
}

func (c *core) handleBlock(block *types.Block) error {

	if err := c.verify(block); err != nil {
		if err == consensus.ErrUnknownAncestor {
			// @todo if expected block is n and what we got is k > n+1, we request 1 at a time. Fix this to request all at once
			c.requestBlock(block.ParentHash(), common.Address{})
			c.unhandledBlocks[block.Hash()] = block
			fmt.Println("Requesting missing block!")
			return nil
		} else {
			c.backend.SendBlame(c.backend.Leader())
			return err
		}
	} // @todo handle potential errors from this

	fmt.Println("Valid block", block.Number().String(), "received!")
	c.progressTimer.AddDuration(2 * c.delta * time.Millisecond)
	c.backend.RelayBlock(block.Hash())

	if len(c.queuedBlocks) == 0 {
		c.commitTimer.Reset(2 * c.delta * time.Millisecond)
		c.nextBlock = block.Hash()
	}

	c.queuedBlocks[block.Hash()] = struct {
		block *types.Block
		time  time.Time
	}{
		block: block,
		time:  time.Now(),
	}

	c.expectedHeight.Add(c.expectedHeight, big.NewInt(1))
	return nil
}

func (c *core) handleRelay(hash common.Hash, addr common.Address) error {
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

func (c *core) handleBlame(addr common.Address) error {
	return nil
}

func (c *core) handleRequest(hash common.Hash, addr common.Address) error {

	block, err := c.backend.GetBlockFromChain(hash)
	if err != nil {
		b, ok := c.queuedBlocks[hash]
		if !ok {
			return errors.New("don't have requested block")
		}
		block = b.block
	}

	go c.backend.RespondToRequest(block, addr)
	return nil
}

func (c *core) handleResponse(block *types.Block) error {

	fmt.Println("handling response for block", block.Number())

	if _, ok := c.requestedBlocks[block.Hash()]; !ok {
		return errors.New("didn't request block")
	}

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
