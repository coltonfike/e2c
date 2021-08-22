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

import "github.com/ethereum/go-ethereum/consensus/e2c"

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
				msg := new(Message)
				if err := msg.FromPayload(ev.Payload, c.checkValidatorSignature); err != nil {
					c.logger.Error("Failed to decode message", "err", err)
				} else if c.handleMsg(msg) {
					c.backend.Broadcast(ev.Payload)
				}
			}

		case block := <-c.blockCh:
			if err := c.propose(block); err != nil {
				c.logger.Error("Failed to propose new block", "err", err)
			}
		case <-c.blockQueue.c():
			if c.backend.Status() != e2c.VotePhase {
				if block, ok := c.blockQueue.getNext(); ok {
					c.commit(block)
				}
			}

		case <-c.progressTimer.c():
			if c.backend.Address() != c.backend.Leader() {
				c.sendBlame()
				c.logger.Info("[E2C] Progress Timer expired! Sending Blame message!")
			}
		case <-c.certTimer.C:
			if c.backend.Status() == e2c.VotePhase {
				if err := c.sendFirstProposal(); err != nil {
					c.logger.Error("Failed to encode block certificate", "err", err)
				}
			}
		}
	}
}

func (c *core) handleMsg(msg *Message) bool {
	// @todo check message came from one of validators

	if err := c.verifyMsg(msg); err != nil {
		c.logger.Error("Failed to verify message", "err", err)
		return false
	}

	switch msg.Code {
	case NewBlockMsg:
		return c.handleProposal(msg)

	case RequestBlockMsg:
		return c.handleRequest(msg)

	case RespondMsg:
		return c.handleResponse(msg)

	case BlameMsg:
		return c.handleBlameMessage(msg)

	case BlameCertificateMsg:
		return c.handleBlameCertificate(msg)

	case VoteMsg:
		return c.handleVote(msg)

	case BlockCertificateMsg:
		return c.handleBlockCertificate(msg)

	case FirstProposalMsg:
		return c.handleFirstProposal(msg)

	case ValidateMsg:
		return c.handleValidate(msg)

	case SecondProposalMsg:
		return c.handleSecondProposal(msg)
	}

	return false
}
