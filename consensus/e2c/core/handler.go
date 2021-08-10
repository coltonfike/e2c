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
			case e2c.MessageEvent:
				if err := c.handleMsg(ev.Payload); err != nil {
					// print err
				}
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
		case <-c.certTimer.C:
			fmt.Println("Timer expired")
			if c.backend.Status() == 1 {
				c.sendB1()
			}
		}
	}
}

func (c *core) handleMsg(payload []byte) error {
	msg := new(e2c.Message)
	if err := msg.FromPayload(payload); err != nil {
		// @todo print error message
		return err
	}

	// @todo check message came from one of validators
	// @todo add backlog stuff ??

	switch msg.Code {
	// @todo add a case for received blamecert
	case e2c.NewBlockMsgCode:
		if msg.Address != c.backend.Leader() {
			return errors.New("errUnauthorized")
		}
		var block *types.Block
		if err := msg.Decode(&block); err != nil {
			return err
		}
		return c.handleBlock(block)

		// @todo remove this
	case e2c.RelayMsgCode:
		var hash common.Hash
		if err := msg.Decode(&hash); err != nil {
			return err
		}
		return c.handleRelay(hash, msg.Address)

	case e2c.BlameMsgCode:
		return c.handleBlameMessage(msg)

	case e2c.BlameCertCode:
		return c.handleCert(msg)

	case e2c.RequestBlockMsgCode:
		return c.handleRequest(msg)

	case e2c.RespondToRequestMsgCode:
		return c.handleResponse(msg)

	case e2c.ValidateMsgCode:
		return c.handleValidate(msg)

	case e2c.VoteMsgCode:
		return c.handleVote(msg)

	case e2c.NewBlockOneMsgCode:
		return c.handleB1(msg)

	case e2c.FinalBlockMsgCode:
		return c.handleB2(msg)

	case e2c.BlockCertMsgCode:
		return c.handleBlockCert(msg)
	}

	return errors.New("Invalid message")
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
