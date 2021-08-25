package core

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/core/types"
)

var (
	errRequestingBlock = errors.New("requesting block")
)

// sends a new block to all the nodes
func (c *core) propose(block *types.Block) error {
	data, err := Encode(block)
	if err != nil {
		return err
	}

	c.broadcast(&Message{
		Code: NewBlockMsg,
		Msg:  data,
	})
	c.lock = block
	c.committed = block
	return nil
}

// handles a new block proposal by verifying it
func (c *core) handleProposal(msg *Message) bool {

	if c.backend.Address() == c.backend.Leader() { // leader doesn't do this process
		return false
	}
	if c.backend.Status() != e2c.SteadyState { // we are currently changing view, ignore all messages
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

	// verify the block is valid
	if err := c.verify(block); err != nil {
		if err == consensus.ErrUnknownAncestor { // blocks may have arrived out of order. request it
			c.blockQueue.insertUnhandled(block)
			if err := c.sendRequest(block.ParentHash(), common.Address{}); err != nil {
				c.logger.Error("Failed to send request", "err", err)
				return err
			}
			return errRequestingBlock // this is necessary to signal an if statement in request that this block wasn't committed
		} else {
			// the block is bad, send blame
			c.logger.Warn("[E2C] Sending Blame", "err", err, "number", block.Number())
			c.sendBlame()
			return err
		}
	}

	// block is good, add to progress timer and insert this block to our queue
	c.logger.Info("[E2C] Valid block received", "number", block.Number().Uint64(), "hash", block.Hash())
	c.progressTimer.AddDuration(2)

	c.blockQueue.insertHandled(block)
	c.lock = block

	return nil
}
