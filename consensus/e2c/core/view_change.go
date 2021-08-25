// This implements the code needed for the view change protocol
package core

import (
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/core/types"
)

// change view
func (c *core) changeView() {

	// reset all our data structures default state
	c.blame = make(map[common.Address]*Message)
	c.validates = make(map[common.Address]*Message)
	c.votes = make(map[common.Hash]map[common.Address]*Message)
	c.progressTimer = NewProgressTimer(c.config.Delta * time.Millisecond) // @todo How to reset this properly? what should timer be reset to? 4 blames on occasion
	c.progressTimer.AddDuration(4)
	c.highestCert = nil

	// store the votes for ourselves and broadcast the blocks so other nodes can vote on them
	c.logger.Info("[E2C] Proposing blocks for voting", "committed number", c.committed.Number(), "committed hash", c.committed.Hash().String(), "lock number", c.lock.Number(), "lock hash", c.lock.Hash())
	c.votes[c.committed.Hash()] = make(map[common.Address]*Message)
	c.votes[c.lock.Hash()] = make(map[common.Address]*Message)

	c.sendVote([]*types.Block{c.committed, c.lock})

	// new leader sets a timer for itself to make first proposal after 4 delta
	if c.backend.Address() == c.backend.Leader() {
		c.certTimer.Reset(4 * c.config.Delta * time.Millisecond)
	}
}

// @todo figure out how to send this with a list of signatures for each block so
// i don't have to use 2 message objects
// Send votes out to all nodes
func (c *core) sendVote(blocks []*types.Block) error {
	// make each vote it's own message. It's ugly, but the quickest way to get it since the receiver needs to store each vote as it's own message
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

	// for all the votes in the message, verify that it's valid
	var myVotes []*types.Block
	for _, vote := range votes {

		var block *types.Block
		if err := vote.Decode(&block); err != nil {
			c.logger.Error("Invalid vote message", "err", err)
			return false
		}

		// if the block falls within our range, we vote on it
		if block.Number().Uint64() >= c.committed.Number().Uint64() && block.Number().Uint64() <= c.lock.Number().Uint64() {

			if b, ok := c.votes[block.Hash()]; ok {
				b[msg.Address] = vote
			}
			c.logger.Info("[E2C] Voted for block", "number", block.Number(), "hash", block.Hash().String())
			myVotes = append(myVotes)
		}
	}
	// send out our votes to all the nodes
	c.sendVote(myVotes)

	// if either lock or committed has enough votes, send the block certificate
	if uint64(len(c.votes[c.committed.Hash()])) == c.backend.F()+1 {
		if err := c.sendBlockCertificate(c.committed); err != nil {
			c.logger.Error("Failed to send block certificate", "err", err)
		}
	}

	if uint64(len(c.votes[c.lock.Hash()])) == c.backend.F()+1 {
		if err := c.sendBlockCertificate(c.lock); err != nil {
			c.logger.Error("Failed to send block certificate", "err", err)
		}
	}

	return true
}

func (c *core) sendBlockCertificate(block *types.Block) error {

	// checkt that this block is the highested certificate locally. otherwise don't send it
	if c.highestCert == nil || c.highestCert.Block.Number().Uint64() < block.Number().Uint64() {
		// attach all the votes it received
		var votes []*Message
		for _, val := range c.votes[block.Hash()] {
			votes = append(votes, val)
		}

		// save it as the new highestCert
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
	// check it has enough votes
	if uint64(len(bc.Votes)) <= c.backend.F() {
		return errors.New("not enough votes")
	}

	// check all the votes are valid
	for _, m := range bc.Votes {
		if err := m.VerifySig(c.checkValidatorSignature); err != nil || m.Code != VoteMsg {
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

	// verify the cert is valid
	if err := c.verifyBlockCertificate(bc); err != nil {
		c.logger.Error("Block certificate invalid", "err", err)
		return false
	}

	c.logger.Info("Block certificate received!", "addr", msg.Address)

	// if it's higher than our previous highest, replace previous highest with this cert
	if c.highestCert == nil || c.highestCert.Block.Number().Uint64() < bc.Block.Number().Uint64() {
		c.highestCert = bc
	}

	return true
}

// sends the first proposal of new view when the 4 delta timer expires
func (c *core) sendFirstProposal() error {

	// commit all the blocks needed to get to the highest cert
	// for example last committed was block 5, highest cert is 10, we commit blocks 5-10 here
	for {
		block, ok := c.blockQueue.getNext()
		if !ok {
			break
		}

		if block.Number().Uint64() <= c.highestCert.Block.Number().Uint64() {
			c.commit(block)
		}
	}

	// set status to allow the engine to make new blocks again
	c.backend.SetStatus(e2c.FirstProposal)
	// read the new block
	block := <-c.blockCh
	//@todo This may be cause of lost block bug
	// set engine to phase for next proposal
	c.backend.SetStatus(e2c.SecondProposal)
	c.logger.Info("[E2C] Proposing new block", "number", block.Number(), "hash", block.Hash().String(), "certificate", c.highestCert)
	// make a new block queue
	c.blockQueue = NewBlockQueue(c.config.Delta)

	data, err := Encode(&FirstProposal{Cert: c.highestCert, Block: block})
	if err != nil {
		return err
	}
	c.broadcast(&Message{
		Code: FirstProposalMsg,
		Msg:  data,
	})

	// validate it's proposal
	m := &Message{Code: ValidateMsg}
	if _, err := c.finalizeMessage(m); err != nil {
		c.logger.Error("Failed to create validate msg")
	}
	c.validates[m.Address] = m

	return nil
}

func (c *core) handleFirstProposal(msg *Message) bool {
	var b FirstProposal
	if err := msg.Decode(&b); err != nil {
		c.logger.Error("Failed to decode first proposal", "err", err)
		return false
	}

	c.logger.Info("[E2C] Proposal for first block in view received", "number", b.Block.Number(), "hash", b.Block.Hash().String())

	// ensure the block cert is valid
	if err := c.verifyBlockCertificate(b.Cert); err != nil {
		c.logger.Error("Block certificate invalid", "err", err)
	}
	// ensure block cert is extending our highest cert
	if b.Cert.Block.Number().Uint64() < c.highestCert.Block.Number().Uint64() {
		c.sendBlame()
		c.logger.Warn("Blame sent", "err", "does not extend block")
		return false
	}

	if err := c.verify(b.Block); err != nil {
		c.logger.Warn("Blame sent", "err", err)
		c.sendBlame()
		return false
	}
	// @todo make this a standalone method so it isn't duplicated
	// commit all blocks up to highest cert
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

	// commit new block in the proposal
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

	// if enough validates are received, send second proposal
	if uint64(len(c.validates)) == c.backend.F()+1 {
		if err := c.sendSecondProposal(); err != nil {
			c.logger.Error("Failed to send second proposal", "err", err)
			return false
		}
	}
	return false
}

func (c *core) sendSecondProposal() error {
	// read block from engine
	block := <-c.blockCh

	// add validates to the proposal
	var validates []*Message
	for _, val := range c.validates {
		validates = append(validates, val)
	}

	data, err := Encode(&SecondProposal{Block: block, Validates: validates})
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
	var b SecondProposal
	if err := msg.Decode(&b); err != nil {
		c.logger.Error("Failed to decode second proposal", "err", err)
		return false
	}

	c.logger.Info("[E2C] Proposal for second block in view received", "number", b.Block.Number(), "hash", b.Block.Hash().String(), "validates", b.Validates)

	// @todo check that each message is from different addr
	if uint64(len(b.Validates)) <= c.backend.F() {
		c.sendBlame()
		c.logger.Warn("Blame sent", "err", "does not contain enough validates")
		return false
	}

	// check that all validates are valid
	for _, m := range b.Validates {
		if err := m.VerifySig(c.checkValidatorSignature); err != nil || m.Code != ValidateMsg {
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
