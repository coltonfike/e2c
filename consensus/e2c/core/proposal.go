package core

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/core/types"
)

// @todo add view to this message

func (c *core) handleProposal(msg *e2c.Message) bool {

	if c.backend.Address() == c.backend.Leader() { // leader doesn't do this process
		return false
	}
	if c.backend.Status() == 1 { // we are currently changing view, ignore all messages
		return false
	}
	if msg.Address != c.backend.Leader() { // message didn't come from leader
		// @todo print err
		return false
	}
	var block *types.Block
	if err := msg.Decode(&block); err != nil { // error decoding the message
		// @todo print err
		return false
	}
	if _, ok := c.blockQueue.get(block.Hash()); ok { // we have already handled this block
		return false
	}
	if err := c.handleBlock(block); err != nil {
		//@todo print err
		return false
	}
	return true
}

func (c *core) handleBlock(block *types.Block) error {

	if err := c.verify(block); err != nil {
		if err == consensus.ErrUnknownAncestor { // blocks may have arrived out of order. request it
			c.blockQueue.insertUnhandled(block)
			if err := c.sendRequest(block.ParentHash(), common.Address{}); err != nil {
				c.logger.Error("Failed to send request", "err", err)
				return err
			}
			return nil
		} else {
			c.logger.Warn("[E2C] Sending Blame", "err", err, "number", block.Number())
			c.sendBlame()
			return err
		}
	}

	c.logger.Info("[E2C] Valid block received", "number", block.Number().Uint64(), "hash", block.Hash())
	c.progressTimer.AddDuration(2 * c.config.Delta * time.Millisecond)

	c.blockQueue.insertHandled(block)
	c.lock = block

	return nil
}
