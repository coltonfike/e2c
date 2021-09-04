package core

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/core/types"
)

// sends a new block to all the nodes
func (c *core) Propose(block *types.Block) error {

	if c.backend.Status() == e2c.Wait {
		return nil
	}

	if c.committed != nil && block.Number().Uint64() != c.committed.Number().Uint64()+1 {
		return errors.New("given duplicate block")
	}
	if c.backend.Status() == e2c.FirstProposal {
		if err := c.sendFirstProposal(block); err != nil {
			return err
		}
		c.committed = block
		return nil
	} else if c.backend.Status() == e2c.SecondProposal {
		if err := c.sendSecondProposal(block); err != nil {
			return err
		}
		c.committed = block
		return nil
	}

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
	if c.blockQueue.contains(block.Hash()) { // we have already handled this block
		return false
	}

	// verify the block is valid
	if err := c.verify(block); err != nil {
		if err == consensus.ErrUnknownAncestor { // blocks may have arrived out of order. Request the block
			c.blockQueue.insertUnhandled(block)
			fmt.Println("Missing parent of block", block.Number().Uint64())
			if err := c.sendRequest(block.ParentHash(), common.Address{}); err != nil {
				c.logger.Error("Failed to send request", "err", err)
			}
			return true
		} else {
			// the block is bad, send blame
			c.logger.Warn("[E2C] Sending Blame", "err", err, "number", block.Number())
			c.sendBlame()
			return false
		}
	}

	c.handleBlockAndAncestors(block)
	return true
}

func (c *core) handleBlockAndAncestors(block *types.Block) error {

	c.handleBlock(block)

	for {
		if child, ok := c.blockQueue.getChild(block.Hash()); ok {

			if err := c.verify(child); err != nil {
				return err
			}

			c.handleBlock(child)
			block = child
		} else {
			return nil
		}
	}
}

func (c *core) handleBlock(block *types.Block) {

	// block is good, add to progress timer and insert this block to our queue
	c.logger.Info("[E2C] Valid block received", "number", block.Number().Uint64(), "hash", block.Hash())
	c.progressTimer.AddDuration(2)
	c.blockQueue.insertHandled(block)
	c.lock = block
}
