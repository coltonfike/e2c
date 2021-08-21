// This implements the code needed for the view change protocol
package core

import (
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/core/types"
)

func (c *core) changeView() {

	c.blame = make(map[common.Address]*Message)
	c.validates = make(map[common.Address]*Message)
	c.votes = make(map[common.Hash]map[common.Address]*Message)
	c.progressTimer = NewProgressTimer(c.config.Delta * time.Millisecond) // @todo How to reset this properly? what should timer be reset to? 4 blames on occasion
	c.progressTimer.AddDuration(4)

	c.logger.Info("[E2C] Proposing blocks for voting", "committed number", c.committed.Number(), "committed hash", c.committed.Hash().String(), "lock number", c.lock.Number(), "lock hash", c.lock.Hash())
	c.votes[c.committed.Hash()] = make(map[common.Address]*Message)
	c.votes[c.lock.Hash()] = make(map[common.Address]*Message)

	c.sendVote([]*types.Block{c.committed, c.lock})

	if c.backend.Address() == c.backend.Leader() {
		c.certTimer.Reset(4 * c.config.Delta * time.Millisecond)
	}
}

// @todo figure out how to send this with a list of signatures for each block so
// i don't have to use 2 message objects
func (c *core) sendVote(blocks []*types.Block) error {
	votes := make([]*Message, len(blocks))
	for i, block := range blocks {
		msg, err := Encode(block)
		if err != nil {
			return err
		}
		v := &Message{
			Code:    VoteMsg,
			Msg:     msg,
			Address: c.backend.Address(),
		}
		v.Sign(c.backend.Sign)
		votes[i] = v
		c.votes[block.Hash()][c.backend.Address()] = v
	}

	msg, err := Encode(votes)
	if err != nil {
		return err
	}

	c.broadcast(&Message{
		Code: VoteMsg,
		Msg:  msg,
	})
	return nil
}

func (c *core) handleVote(msg *Message) bool {

	var votes []*Message
	if err := msg.Decode(&votes); err != nil {
		c.logger.Error("Failed to decode vote message", "err", err)
		return false
	}

	var myVotes []*types.Block
	for _, vote := range votes {

		var block *types.Block
		if err := vote.Decode(&block); err != nil {
			c.logger.Error("Invalid vote message", "err", err)
			return false
		}

		if block.Number().Uint64() >= c.committed.Number().Uint64() && block.Number().Uint64() <= c.lock.Number().Uint64() {

			if b, ok := c.votes[block.Hash()]; ok {
				b[msg.Address] = vote
			}
			c.logger.Info("[E2C] Voted for block", "number", block.Number(), "hash", block.Hash().String())
			myVotes = append(myVotes)
		}
	}
	c.sendVote(myVotes)

	if uint64(len(c.votes[c.committed.Hash()])) > c.config.F {
		if err := c.sendBlockCertificate(c.committed); err != nil {
			c.logger.Error("Failed to send block certificate", "err", err)
		}
	}
	if uint64(len(c.votes[c.lock.Hash()])) > c.config.F {
		if err := c.sendBlockCertificate(c.lock); err != nil {
			c.logger.Error("Failed to send block certificate", "err", err)
		}
	}

	return true
}

func (c *core) sendBlockCertificate(block *types.Block) error {
	if c.highestCert == nil || c.highestCert.Block.Number().Uint64() < block.Number().Uint64() {
		var votes []*Message
		for _, val := range c.votes[block.Hash()] {
			votes = append(votes, val)
		}

		c.highestCert = &BlockCertificate{
			Block: block,
			Votes: votes,
		}

		c.logger.Info("[E2C] New Highest Block is certified!", "number", block.Number(), "hash", block.Hash().String(), "votes", votes)

		m, err := Encode(c.highestCert)
		if err != nil {
			return err
		}
		c.broadcast(&Message{
			Code: BlockCertificateMsg,
			Msg:  m,
		})
	}
	return nil
}

func (c *core) verifyBlockCertificate(bc *BlockCertificate) error {
	if uint64(len(bc.Votes)) <= c.config.F {
		return errors.New("not enough votes")
	}
	for _, m := range bc.Votes {
		if err := m.VerifySig(); err != nil || m.Code != VoteMsg {
			return errors.New("votes message invalid")
		}
	}
	return nil
}

func (c *core) handleBlockCertificate(msg *Message) bool {
	var bc *BlockCertificate
	if err := msg.Decode(&bc); err != nil {
		c.logger.Error("Failed to decode block certificate", "err", err)
		return false
	}

	if err := c.verifyBlockCertificate(bc); err != nil {
		c.logger.Error("Block certificate invalid", "err", err)
		return false
	}

	c.logger.Info("Block certificate received!", "addr", msg.Address)

	if c.highestCert == nil || c.highestCert.Block.Number().Uint64() < bc.Block.Number().Uint64() {
		c.highestCert = bc
	}

	return true
}

func (c *core) sendFirstProposal() error {

	for {
		block, ok := c.blockQueue.getNext()
		if !ok {
			break
		}

		if block.Number().Uint64() <= c.highestCert.Block.Number().Uint64() {
			c.commit(block)
		}
	}

	c.backend.SetStatus(e2c.FirstProposal)
	block := <-c.blockCh
	c.backend.SetStatus(e2c.SecondProposal)
	c.logger.Info("[E2C] Proposing new block", "number", block.Number(), "hash", block.Hash().String(), "certificate", c.highestCert)
	c.blockQueue = NewBlockQueue(c.config.Delta)

	data, err := Encode(&B1{Cert: c.highestCert, Block: block})
	if err != nil {
		return err
	}
	c.broadcast(&Message{
		Code: FirstProposalMsg,
		Msg:  data,
	})
	return nil
}

func (c *core) handleFirstProposal(msg *Message) bool {
	var b B1
	if err := msg.Decode(&b); err != nil {
		c.logger.Error("Failed to decode first proposal", "err", err)
		return false
	}

	c.logger.Info("[E2C] Proposal for first block in view received", "number", b.Block.Number(), "hash", b.Block.Hash().String())

	if err := c.verifyBlockCertificate(b.Cert); err != nil {
		c.logger.Error("Block certificate invalid", "err", err)
	}
	if b.Cert.Block.Number().Uint64() < c.highestCert.Block.Number().Uint64() {
		c.sendBlame()
		c.logger.Warn("Blame sent", "err", "does not extend block")
		return false
	}

	// @todo FIX THIS VERIFY BECAUSE IT ALWAYS BLAMES
	if err := c.verify(b.Block); err != nil {
		c.logger.Warn("Blame sent", "err", err)
		c.sendBlame()
		return false
	}
	// @todo make this a standalone method so it isn't duplicated
	for {
		block, ok := c.blockQueue.getNext()
		if !ok {
			break
		}

		if block.Number().Uint64() <= c.highestCert.Block.Number().Uint64() {
			c.commit(block)
		}
	}
	c.blockQueue = NewBlockQueue(c.config.Delta)

	c.blockQueue.insertHandled(b.Block)
	c.lock = b.Block

	// @todo is this a broadcast or just send to leader?
	c.send(&Message{
		Code: ValidateMsg,
	}, c.backend.Leader())

	c.backend.SetStatus(e2c.SecondProposal)
	c.logger.Info("[E2C] Sending validate for first block in view", "number", b.Block.Number(), "hash", b.Block.Hash().String())
	return true
}

// @todo maybe broadcast these?
func (c *core) handleValidate(msg *Message) bool {

	c.logger.Info("[E2C] Received validate message", "addr", msg.Address)
	c.validates[msg.Address] = msg

	// @todo replace with F, but can't now because node 1 dies when it's bad
	// @todo maybe leader validates it's own message so that can get F sigs?
	if uint64(len(c.validates)) == 2 {
		if err := c.sendSecondProposal(); err != nil {
			c.logger.Error("Failed to send second proposal", "err", err)
			return false
		}
	}
	return false
}

func (c *core) sendSecondProposal() error {
	block := <-c.blockCh

	var validates []*Message
	for _, val := range c.validates {
		validates = append(validates, val)
	}

	data, err := Encode(&B2{Block: block, Validates: validates})
	if err != nil {
		return err
	}
	c.broadcast(&Message{
		Code: SecondProposalMsg,
		Msg:  data,
	})
	c.logger.Info("[E2C] Sent proposal for second block in view", "number", block.Number(), "hash", block.Hash().String(), "validates", validates)
	c.backend.SetStatus(e2c.SteadyState)
	return nil
}

func (c *core) handleSecondProposal(msg *Message) bool {
	var b B2
	if err := msg.Decode(&b); err != nil {
		c.logger.Error("Failed to decode second proposal", "err", err)
		return false
	}

	c.logger.Info("[E2C] Proposal for second block in view received", "number", b.Block.Number(), "hash", b.Block.Hash().String(), "validates", b.Validates)

	// @todo check that each message is from different addr
	// @todo change 2 to F when we get it working
	// @todo maybe leader validates it's own message so that can get F sigs?
	if uint64(len(b.Validates)) < 2 {
		c.sendBlame()
		c.logger.Warn("Blame sent", "err", "does not contain enough validates")
		return false
	}
	for _, m := range b.Validates {
		if err := m.VerifySig(); err != nil || m.Code != ValidateMsg {
			c.sendBlame()
			c.logger.Warn("Blame sent", "err", "validates not valid")
			return false
		}
	}

	if err := c.verify(b.Block); err != nil {
		c.logger.Warn("Blame sent", "err", "invalid block")
		c.sendBlame()
		return false
	}
	c.blockQueue.insertHandled(b.Block)
	c.lock = b.Block
	c.backend.SetStatus(e2c.SteadyState)
	c.logger.Info("[E2C] View Change completed! Resuming normal operations")
	return true
}
