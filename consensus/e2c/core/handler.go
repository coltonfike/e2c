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
	"sync/atomic"
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
				c.handleBlame(ev.Time, ev.Address)
			case e2c.BlameCertificateEvent:
				c.handleCert(ev.Lock, ev.Committed, ev.Address)
			case e2c.BlockCertificateEvent:
				c.handleBlockCert(ev.Block)
			case e2c.Vote:
				c.handleVote(ev.Block, ev.Address)
			case e2c.RequestBlockEvent:
				c.handleRequest(ev.Hash, ev.Address)
			case e2c.RespondToRequestEvent:
				c.handleResponse(ev.Block)
			case e2c.ValidateEvent:
				c.handleValidate(ev.Address)

			case e2c.B1:
				c.handleB1(ev)
			case e2c.B2:
				c.handleB2(ev)
			}

		case <-c.blockQueue.c():
			if c.backend.Status() != 1 {
				if block, ok := c.blockQueue.getNext(); ok {
					c.commit(block)
				}
			}

		case <-c.progressTimer.Chan():
			if c.backend.Address() != c.backend.Leader() {
				c.sendBlame()
				c.logger.Info("Progress Timer expired! Sending Blame message!")
			}
		}
	}
}

func (c *core) handleBlock(block *types.Block) error {
	if c.backend.Status() == 1 {
		return nil
	}

	if _, ok := c.blockQueue.get(block.Hash()); ok {
		return nil
	}

	if err := c.verify(block); err != nil {
		if err == consensus.ErrUnknownAncestor {

			// @todo if expected block is n and what we got is k > n+1, we request 1 at a time. Fix this to request all at once

			c.blockQueue.insertUnhandled(block)
			c.requestBlock(block.ParentHash(), common.Address{})
			c.logger.Info("Requesting missing block", "hash", block.ParentHash())
			return errors.New("Requesting")
		} else {
			c.logger.Warn("Sending Blame", "err", err, "number", block.Number())
			c.sendBlame()
			return err
		}
	}

	c.logger.Info("Valid block received", "number", block.Number().Uint64(), "hash", block.Hash())
	c.progressTimer.AddDuration(2 * c.config.Delta * time.Millisecond)
	c.backend.RelayBlock(block.Hash())

	c.blockQueue.insertHandled(block)
	c.lock = block

	return nil
}

func (c *core) handleRelay(hash common.Hash, addr common.Address) error {

	// if any of there are true, we have the block
	c.logger.Debug("Relay received", "hash", hash, "address", addr)
	if _, err := c.backend.GetBlockFromChain(hash); err == nil {
		return nil
	}
	if c.blockQueue.contains(hash) || c.blockQueue.hasRequest(hash) {
		return nil
	}

	c.requestBlock(hash, addr)
	return nil
}

func (c *core) handleBlame(t time.Time, addr common.Address) error {

	if c.backend.Address() == c.backend.Leader() {
		return nil
	}

	if _, ok := c.blame[addr]; !ok {
		c.blame[addr] = struct{}{}
	}

	c.logger.Info("Blame message received", "total blame", len(c.blame))
	if uint64(len(c.blame)) >= c.config.F {
		atomic.StoreUint32(&c.viewChange, 1)
		c.backend.ChangeView()
		if c.backend.Address() != c.backend.Leader() {
			c.vote[c.lock.Hash()] = 1
			c.vote[c.committed.Hash()] = 1
			c.backend.SendBlameCertificate(e2c.BlameCertificate{Lock: c.lock, Committed: c.committed})
		}
	}
	return nil
}

func (c *core) handleCert(lock *types.Block, committed *types.Block, addr common.Address) {
	fmt.Println("Handling Cert!")
	if committed.Number().Uint64() >= c.committed.Number().Uint64() && committed.Number().Uint64() <= c.lock.Number().Uint64() {
		fmt.Println("Sent vote")
		c.backend.SendVote(committed, addr)
	}
	if lock.Number().Uint64() >= c.committed.Number().Uint64() && lock.Number().Uint64() <= c.lock.Number().Uint64() {
		fmt.Println("Sent vote")
		c.backend.SendVote(lock, addr)
	}
}

func (c *core) handleVote(block *types.Block, addr common.Address) {
	c.vote[block.Hash()]++
	fmt.Println("Vote received!!")
	if c.vote[block.Hash()] > c.config.F {
		if c.highestCert == nil {
			c.highestCert = block
			fmt.Println("Block", block.Number().Uint64(), "certified. Sending certificate to new leader!")
			c.backend.SendBlockCert(c.highestCert)
		} else if c.highestCert.Number().Uint64() < block.Number().Uint64() {
			c.highestCert = block
			fmt.Println("Block", block.Number().Uint64(), "certified. Sending certificate to new leader!")
			c.backend.SendBlockCert(c.highestCert)
		}
	}
}

func (c *core) handleBlockCert(block *types.Block) {
	c.certReceived++
	fmt.Println("BlockCert Received")
	if c.highestCert == nil {
		c.highestCert = block
	} else if c.highestCert.Number().Uint64() < block.Number().Uint64() {
		c.highestCert = block
	}

	// @todo change this to a timer?
	if uint64(c.certReceived) >= 4 {
		fmt.Println("Highest is", c.highestCert.Number())

		for {
			block, ok := c.blockQueue.getNext()
			if !ok {
				break
			}
			fmt.Println(block.Number())

			if block.Number().Uint64() <= c.highestCert.Number().Uint64() {
				c.commit(block)
			}
		}

		c.backend.SetStatus(2)
		block := <-c.ch
		c.backend.SetStatus(3)
		fmt.Println("Block", block.Number())
		c.blockQueue.clear()
		c.backend.SendBlockOne(e2c.B1{Cert: c.highestCert, Block: block})
	}
}

func (c *core) handleRequest(hash common.Hash, addr common.Address) error {

	c.logger.Debug("Request Received", "hash", hash, "address", addr)
	block, err := c.backend.GetBlockFromChain(hash)
	if err != nil {
		p, ok := c.blockQueue.get(hash)
		if !ok {
			c.logger.Debug("Don't have requested block", "hash", hash, "address", addr)
			return errors.New("don't have requested block")
		}
		block = p
	}

	go c.backend.RespondToRequest(block, addr)
	return nil
}

func (c *core) handleResponse(block *types.Block) error {

	if !c.blockQueue.hasRequest(block.Hash()) {
		return nil
	}

	c.logger.Info("Response to request received", "number", block.Number().Uint64(), "hash", block.Hash())

	if err := c.handleBlock(block); err != nil {
		return err
	}
	delete(c.blockQueue.requestQueue, block.Hash())

	for {
		if child, ok := c.blockQueue.getChild(block.Hash()); ok {
			if err := c.handleBlock(child); err != nil {
				return err
			}
			block = child
		} else {
			break
		}
	}
	return nil
}

func (c *core) handleB1(b e2c.B1) {
	if b.Cert.Number().Uint64() < c.highestCert.Number().Uint64() {
		// @todo also check votes are correct
		c.backend.SendBlame()
	}
	for {
		block, ok := c.blockQueue.getNext()
		if !ok {
			break
		}
		fmt.Println(block.Number())

		if block.Number().Uint64() <= c.highestCert.Number().Uint64() {
			c.commit(block)
		}
	}
	c.blockQueue.clear()
	if err := c.verify(b.Block); err != nil {
		c.backend.SendBlame()
	}
	c.blockQueue.insertHandled(b.Block)
	c.lock = b.Block
	c.backend.SendValidate()
	c.backend.SetStatus(2)
	fmt.Println("Check 1 Passed, sending Ack")
}

func (c *core) handleValidate(addr common.Address) {
	fmt.Println("Received validate")
	c.validates[addr] = struct{}{}
	// @todo replace with F, but can't now because node 1 dies when it's bad
	if uint64(len(c.validates)) >= 2 {
		fmt.Println("Trying to read from ch")
		block := <-c.ch
		c.backend.SendFinal(e2c.B2{Validates: block, Block: block})
		fmt.Println("Sent final")
		c.backend.SetStatus(0)
	}
}

func (c *core) handleB2(b e2c.B2) {

	// @todo do error checking here

	fmt.Println("Recieved final block!")
	if err := c.verify(b.Block); err != nil {
		c.backend.SendBlame()
	}
	c.blockQueue.insertHandled(b.Block)
	c.lock = b.Block
	c.backend.SetStatus(0)
}
