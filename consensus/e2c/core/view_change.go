// This implements the code needed for the view change protocol
package core

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

// change view
func (c *core) changeView() {

	// reset all our data structures default state
	c.blame = make(map[common.Address][]byte)
	c.validates = make(map[common.Address][]byte)
	c.votes = make(map[common.Hash]map[common.Address][]byte)
	// TODO what should the timer be set to in view change?
	c.progressTimer = NewProgressTimer(8 * c.config.Delta * time.Millisecond)
	c.highestCert = nil

	// store the votes for ourselves and broadcast the blocks so other nodes can vote on them
	log.Info("Proposing blocks for voting", "committed number", c.committed.Number(), "committed hash", c.committed.Hash(), "lock number", c.lock.Number(), "lock hash", c.lock.Hash())
	c.votes[c.committed.Hash()] = make(map[common.Address][]byte)
	c.votes[c.lock.Hash()] = make(map[common.Address][]byte)

	c.sendVote([]*types.Block{c.committed, c.lock})

	// new leader sets a timer for itself to make first proposal after 4 delta
	if c.backend.Address() == c.backend.Leader() {
		c.votingTimer.Reset(4 * c.config.Delta * time.Millisecond)
	}
}

// Send votes out to all nodes
func (c *core) sendVote(blocks []*types.Block) error {
	// make each vote it's own message. It's ugly, but the easiest way to get signatures on independent blocks rather than the whole message
	votes := make([]*Message, len(blocks))
	for i, block := range blocks {
		msg, err := Encode(block)
		if err != nil {
			return err
		}
		v := &Message{
			Code: VoteMsg,
			Msg:  msg,
			View: c.backend.View(),
		}
		v.Sign(c.backend.Sign)
		votes[i] = v
		c.votes[block.Hash()][c.backend.Address()] = v.Signature
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
		log.Error("Failed to decode vote message", "err", err)
		return false
	}

	// for all the votes in the message, verify that it's valid
	var myVotes []*types.Block
	for _, vote := range votes {

		var block *types.Block
		if err := vote.Decode(&block); err != nil {
			log.Error("Invalid vote message", "err", err)
			return false
		}

		// if the block falls within our range, we vote on it
		if block.Number().Uint64() >= c.committed.Number().Uint64() && block.Number().Uint64() <= c.lock.Number().Uint64() {

			if b, ok := c.votes[block.Hash()]; ok {
				b[msg.Address] = vote.Signature
			}
			log.Info("Voted for block", "number", block.Number(), "hash", block.Hash())
			myVotes = append(myVotes)
		}
	}
	// send out our votes to all the nodes
	c.sendVote(myVotes)

	// if either lock or committed has enough votes, send the block certificate
	if uint64(len(c.votes[c.committed.Hash()])) == c.backend.F()+1 {
		if err := c.sendBlockCertificate(c.committed); err != nil {
			log.Error("Failed to send block certificate", "err", err)
		}
	}

	if uint64(len(c.votes[c.lock.Hash()])) == c.backend.F()+1 {
		if err := c.sendBlockCertificate(c.lock); err != nil {
			log.Error("Failed to send block certificate", "err", err)
		}
	}

	return true
}

func (c *core) sendBlockCertificate(block *types.Block) error {
	// check that this block is the highested certificate locally. otherwise don't send it
	if c.highestCert == nil || c.highestCert.Block.Number().Uint64() < block.Number().Uint64() {
		// attach all the votes it received
		var votes [][]byte
		for _, val := range c.votes[block.Hash()] {
			votes = append(votes, val)
		}

		// save it as the new highestCert
		c.highestCert = &BlockCertificate{
			Block: block,
			Votes: votes,
		}

		log.Info("New Highest Block is certified!", "number", block.Number(), "hash", block.Hash())

		// send the certificate to all nodes
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
		return errNotEnoughSignatures
	}

	m, err := Encode(&bc.Block)
	if err != nil {
		return err
	}
	msg := &Message{
		Code: VoteMsg,
		View: c.backend.View(),
		Msg:  m,
	}

	// check all the votes are valid
	return VerifyCertificateSignatures(msg, bc.Votes, c.checkValidatorSignature)
}

func (c *core) handleBlockCertificate(msg *Message) bool {
	var bc *BlockCertificate
	if err := msg.Decode(&bc); err != nil {
		log.Error("Failed to decode block certificate", "err", err)
		return false
	}

	// verify the cert is valid
	if err := c.verifyBlockCertificate(bc); err != nil {
		log.Error("Block certificate invalid", "err", err)
		return false
	}

	log.Debug("Block certificate received!", "addr", msg.Address)

	// if it's higher than our previous highest, replace previous highest with this cert
	if c.highestCert == nil || c.highestCert.Block.Number().Uint64() < bc.Block.Number().Uint64() {
		c.highestCert = bc
		log.Info("New Highest Block is certified!", "number", c.highestCert.Block.Number(), "hash", c.highestCert.Block.Hash())
	}

	return true
}

func (c *core) prepareFirstProposal() {

	// commit all the blocks needed to get to the highest cert
	// for example last committed was block 5, highest cert is 10, we commit blocks 5-10 here
	c.commitToHighest()
	c.blockQueue = NewBlockQueue(c.config.Delta)
	c.backend.SetStatus(e2c.FirstProposal)
}

// sends the first proposal of new view when the 4 delta timer expires
func (c *core) sendFirstProposal(block *types.Block) error {

	log.Info("Proposing first block in view", "number", block.Number(), "hash", block.Hash())

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
		log.Error("Failed to create validate msg")
	}
	c.validates[m.Address] = m.Signature

	c.backend.SetStatus(e2c.Wait)
	return nil
}

func (c *core) handleFirstProposal(msg *Message) bool {
	var b FirstProposal
	if err := msg.Decode(&b); err != nil {
		log.Error("Failed to decode first proposal", "err", err)
		return false
	}

	log.Info("Proposal for first block in view received", "number", b.Block.Number(), "hash", b.Block.Hash())

	// ensure the block cert is valid
	if err := c.verifyBlockCertificate(b.Cert); err != nil {
		c.sendBlame()
		log.Warn("Blame sent", "err", err)
	}
	// ensure block cert is extending our highest cert
	if b.Cert.Block.Number().Uint64() < c.highestCert.Block.Number().Uint64() {
		c.sendBlame()
		log.Warn("Blame sent", "err", errInvalidBlockCertificate)
		return false
	}

	c.lock = b.Cert.Block

	if err := c.verify(b.Block); err != nil {
		c.sendBlame()
		log.Warn("Blame sent", "err", errInvalidBlock)
		return false
	}

	// commit all blocks up to highest cert
	c.commitToHighest()

	// commit new block in the proposal
	c.handleBlock(b.Block)

	c.broadcast(&Message{
		Code: ValidateMsg,
	})

	log.Info("Sending validate for first block in view", "number", b.Block.Number(), "hash", b.Block.Hash())
	return true
}

func (c *core) handleValidate(msg *Message) bool {
	// only the leader needs to use check these
	if c.backend.Address() != c.backend.Leader() {
		return true
	}

	log.Info("Received validate message", "addr", msg.Address)
	c.validates[msg.Address] = msg.Signature

	// if enough validates are received, send second proposal
	if uint64(len(c.validates)) == c.backend.F()+1 {
		c.backend.SetStatus(e2c.SecondProposal)
	}
	return false
}

func (c *core) sendSecondProposal(block *types.Block) error {

	// add validates to the proposal
	var validates [][]byte
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

	log.Info("Sent proposal for second block in view", "number", block.Number(), "hash", block.Hash())
	// view change is over, set the state back to normal
	c.backend.SetStatus(e2c.SteadyState)
	return nil
}

func (c *core) handleSecondProposal(msg *Message) bool {
	var b SecondProposal
	if err := msg.Decode(&b); err != nil {
		log.Error("Failed to decode second proposal", "err", err)
		return false
	}

	log.Info("Proposal for second block in view received", "number", b.Block.Number(), "hash", b.Block.Hash())

	if uint64(len(b.Validates)) <= c.backend.F() {
		c.sendBlame()
		log.Warn("Blame sent", "err", errNotEnoughSignatures)
		return false
	}

	// check that all validates are valid
	m := &Message{
		Code: ValidateMsg,
		View: c.backend.View(),
	}
	if err := VerifyCertificateSignatures(m, b.Validates, c.checkValidatorSignature); err != nil {
		c.sendBlame()
		log.Warn("Blame sent", "err", errInvalidValidates)
		return false
	}

	if err := c.verify(b.Block); err != nil {
		log.Warn("Blame sent", "err", errInvalidBlock)
		c.sendBlame()
		return false
	}

	c.handleBlock(b.Block)
	c.backend.SetStatus(e2c.SteadyState)
	log.Info("View Change completed! Resuming normal operations")
	return true
}

// this will commit all the blocks up to the highest certificate then reset the block queue
func (c *core) commitToHighest() {
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
}
