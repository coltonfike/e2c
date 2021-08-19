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
			case e2c.MessageEvent:
				msg := new(e2c.Message)
				if err := msg.FromPayload(ev.Payload); err != nil {
					// @todo print error message
				} else if c.handleMsg(msg) {
					c.backend.Broadcast(ev.Payload)
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
				c.logger.Info("[E2C] Progress Timer expired! Sending Blame message!")
			}
		case <-c.certTimer.C:
			if c.backend.Status() == 1 {
				if err := c.sendFirstProposal(); err != nil {
					c.logger.Error("Failed to encode block certificate", "err", err)
				}
			}
		}
	}
}

func (c *core) handleMsg(msg *e2c.Message) bool {
	// @todo check message came from one of validators

	switch msg.Code {
	// @todo add a case for received blamecert
	case e2c.NewBlockMsgCode:
		return c.handleProposal(msg)

	case e2c.RequestBlockMsgCode:
		return c.handleRequest(msg)

	case e2c.RespondToRequestMsgCode:
		return c.handleResponse(msg)

	case e2c.BlameMsgCode:
		return c.handleBlameMessage(msg)

	case e2c.BlameCertCode:
		return c.handleBlameCertificate(msg)

	case e2c.VoteMsgCode:
		return c.handleVote(msg)

	case e2c.BlockCertMsgCode:
		return c.handleBlockCertificate(msg)

	case e2c.NewBlockOneMsgCode:
		return c.handleFirstProposal(msg)

	case e2c.ValidateMsgCode:
		return c.handleValidate(msg)

	case e2c.FinalBlockMsgCode:
		return c.handleSecondProposal(msg)
	}

	return false
}
