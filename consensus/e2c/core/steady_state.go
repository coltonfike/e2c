package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

// sends a new block to all the nodes
func (c *core) Propose(block *types.Block) error {

	if c.backend.Status() == e2c.Wait {
		return nil
	}

	// we need to check this here. Eth engine will give duplicate blocks if it gets more transactions
	// TODO last check is to allow the leader to equivocate, it MUST be removed for real environments
	if c.committed != nil && block.Number().Uint64() != c.committed.Number().Uint64()+1 && c.backend.Address() != c.backend.Validators()[0] {
		return errDuplicateBlock
	}

	// check if this proposal is a special case and handle accordingly
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

	// reqular proposal, send the block to all nodes
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

	// leader shouldn't handle these
	if c.backend.Address() == c.backend.Leader() {
		return false
	}
	// we ignore these if we aren't in steady state. The special cases are handled in view_change.go
	if c.backend.Status() != e2c.SteadyState {
		return false
	}
	// message didn't come from leader
	if msg.Address != c.backend.Leader() {
		return false
	}
	var block *types.Block
	if err := msg.Decode(&block); err != nil {
		log.Error("Failed to decode proposal", "err", err)
		return false
	}
	if c.blockQueue.contains(block.Hash()) { // we have already handled this block
		return false
	}

	// verify the block is valid
	if err := c.verify(block); err != nil {
		// blocks may have arrived out of order or this node somehow missed a block (possibly joined the network late)
		// either way, we should request the block to attempt to recover
		if err == consensus.ErrUnknownAncestor {
			c.blockQueue.insertUnhandled(block)
			if err := c.sendRequest(block.ParentHash(), common.Address{}); err != nil {
				log.Error("Failed to send request", "err", err)
			}
			return true

			// the block is bad, send blame
		} else {
			// equivocation is a special case that requires a unique message
			if err == errEquivocatingBlocks {
				equivBlock, ok := c.blockQueue.getByNumber(block.Number().Uint64())
				if !ok {
					equivBlock = c.backend.GetBlockByNumber(block.Number().Uint64())
				}
				log.Warn("Sending Blame", "err", err, "number", block.Number(), "B1 hash", block.Hash(), "B2 hash", equivBlock.Hash())
				c.sendEquivBlame(block, equivBlock)
				return false
			}
			log.Warn("Sending Blame", "err", err, "number", block.Number())
			c.sendBlame()
			return false
		}
	}

	// the block is valid, insert it into the queue!
	c.handleBlockAndAncestors(block)
	return true
}

// places a block into the queue, then checks if we have any of the blocks ancestors waiting to be handled
// that could happen when blocks arrive out of order
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

// handles a single block by adding it to the queue and extending the progress timer
func (c *core) handleBlock(block *types.Block) {

	// block is good, add to progress timer and insert this block to our queue
	log.Info("Valid block received", "number", block.Number().Uint64(), "hash", block.Hash())
	c.progressTimer.AddDuration(2)
	c.blockQueue.insertHandled(block)
	c.lock = block
}
