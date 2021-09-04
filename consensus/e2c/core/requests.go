// When starting the protocol, ethereum synching doesn't always work
// We use this to sync on our own. It's also used in cases where blocks arrive
// out of order, though that doesn't happen often
package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// @todo add a timeout feature!
func (c *core) sendRequest(hash common.Hash, addr common.Address) error {
	c.blockQueue.insertRequest(hash)

	data, err := Encode(hash)
	if err != nil {
		return err
	}

	c.logger.Debug("Requesting missing block", "hash", hash)
	c.broadcast(&Message{
		Code: RequestBlockMsg,
		Msg:  data,
	})
	return nil
}

// handles a request from another node for a specific block
func (c *core) handleRequest(msg *Message) bool {

	var hash common.Hash
	if err := msg.Decode(&hash); err != nil {
		c.logger.Error("Failed to decode request", "err", err)
		return false
	}
	c.logger.Debug("Request for block received", "hash", hash, "from", msg.Address)

	block, err := c.backend.GetBlockFromChain(hash) // check if the block has been committed
	if err != nil {                                 // if it hasn't been committed, check if we have the block in our queue
		p, ok := c.blockQueue.get(hash)
		if !ok {
			c.logger.Debug("Don't have the requested block", "hash", hash, "from", msg.Address)
			return true
		}
		block = p
	}

	data, err := Encode(block)
	if err != nil {
		c.logger.Error("Failed to encode response", "err", err)
		return true
	}

	go c.send(&Message{
		Code: RespondMsg,
		Msg:  data,
	}, msg.Address) // if we have the block, send it to the node that requested it
	return false
}

// handles a response to a request for a block
func (c *core) handleResponse(msg *Message) bool {
	var block *types.Block
	if err := msg.Decode(&block); err != nil {
		c.logger.Error("Failed to decode response", "err", err)
		return false
	}

	// check if we requested it
	if !c.blockQueue.hasRequest(block.Hash()) {
		return false // don't return an error, as we may have requested the block, but since then we received the block and handled it properly
	}
	c.logger.Info("Response to request received", "number", block.Number().Uint64(), "hash", block.Hash(), "from", msg.Address)

	c.handleBlockAndAncestors(block)
	delete(c.blockQueue.requestQueue, block.Hash())
	return false
}
