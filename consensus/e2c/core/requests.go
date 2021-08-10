// When starting the protocol, ethereum synching doesn't always work
// We use this to sync on our own. It's also used in cases where blocks arrive
// out of order, though that doesn't happen often
package core

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/core/types"
)

// handles a request from another node for a specific block
func (c *core) handleRequest(msg *e2c.Message) error {

	var hash common.Hash
	if err := msg.Decode(&hash); err != nil {
		return err
	}
	c.logger.Debug("Request from block received", "hash", hash, "from", msg.Address)

	block, err := c.backend.GetBlockFromChain(hash) // check if the block has been committed
	if err != nil {                                 // if it hasn't been committed, check if we have the block in our queue
		p, ok := c.blockQueue.get(hash)
		if !ok {
			c.logger.Debug("Don't have the requested block", "hash", hash, "from", msg.Address)
			return errors.New("don't have requested block")
		}
		block = p
	}

	go c.backend.RespondToRequest(block, msg.Address) // if we have the block, send it to the node that requested it
	return nil
}

// handles a response to a request for a block
func (c *core) handleResponse(msg *e2c.Message) error {
	var block *types.Block
	if err := msg.Decode(&block); err != nil {
		return err
	}

	// check if we requested it
	if !c.blockQueue.hasRequest(block.Hash()) {
		return nil // don't return an error, as we may have requested the block, but since then we received the block and handled it properly
	}
	c.logger.Info("Response to request received", "number", block.Number().Uint64(), "hash", block.Hash(), "from", msg.Address)

	if err := c.handleBlock(block); err != nil {
		return err
	}
	delete(c.blockQueue.requestQueue, block.Hash())

	// if this block was missing, it probably stopped us from committing the block coming after it (say we missed block 4, but we do have 5,6,..).
	// so we look to see if we have this blocks child then handle that block as well. Continue until we have handled all the children
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
