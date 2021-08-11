// This implements the code needed for the view change protocol
package core

import (
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/core/types"
)

func (c *core) handleBlameMessage(msg *e2c.Message) error {
	var t time.Time
	if err := msg.Decode(&t); err != nil {
		return err
	}

	// @todo maybe remove this?
	if c.backend.Address() == c.backend.Leader() {
		return nil
	}

	c.blame[msg.Address] = msg // add this message to our blame map

	c.logger.Info("Blame message received", "total blame", len(c.blame))

	// @todo send blame certificate, then wait 1 delta, then send votes..
	// @todo handle blame cert!
	if uint64(len(c.blame)) >= c.config.F {
		if c.backend.Status() != 0 {
			return nil
		}
		atomic.StoreUint32(&c.viewChange, 1)
		c.backend.ChangeView()

		var blames []*e2c.Message
		for _, m := range c.blame {
			blames = append(blames, m)
		}
		msg, err := e2c.Encode(blames)
		if err != nil {
			return err
		}
		bc := &e2c.Message{
			Code:    e2c.RBlameCertCode,
			Msg:     msg,
			Address: c.backend.Address(),
		}
		p, err := bc.PayloadWithSig(c.backend.Sign)
		if err != nil {
			return err
		}
		c.backend.Broadcast(p)
		<-time.After(c.config.Delta * time.Millisecond)

		msg, err = e2c.Encode(c.committed)
		if err != nil {
			return err
		}
		v1 := &e2c.Message{
			Code:    e2c.VoteMsgCode,
			Msg:     msg,
			Address: c.backend.Address(),
		}
		v1.Sign(c.backend.Sign)

		msg, err = e2c.Encode(c.lock)
		if err != nil {
			return err
		}
		v2 := &e2c.Message{
			Code:    e2c.VoteMsgCode,
			Msg:     msg,
			Address: c.backend.Address(),
		}
		v2.Sign(c.backend.Sign)

		c.logger.Info("Proposing blocks for voting", "committed number", c.committed.Number(), "committed hash", c.committed.Hash(), "lock number", c.lock.Number(), "lock hash", c.lock.Hash())
		c.vote[c.committed.Hash()] = make(map[common.Address]*e2c.Message)
		c.vote[c.lock.Hash()] = make(map[common.Address]*e2c.Message)
		c.vote[c.committed.Hash()][c.backend.Address()] = v1
		c.vote[c.lock.Hash()][c.backend.Address()] = v2
		c.backend.SendBlameCertificate(e2c.BlameCertificate{Lock: c.lock, Committed: c.committed})
		if c.backend.Address() == c.backend.Leader() {
			c.certTimer.Reset(4 * c.config.Delta * time.Millisecond)
		}
	}
	return nil
}

func (c *core) handleCert(msg *e2c.Message) error {
	var bc e2c.BlameCertificate
	if err := msg.Decode(&bc); err != nil {
		return err
	}
	c.logger.Info("Received proposal for new block", "addr", msg.Address)
	if bc.Committed.Number().Uint64() >= c.committed.Number().Uint64() && bc.Committed.Number().Uint64() <= c.lock.Number().Uint64() {
		c.logger.Info("Voted for block", "number", bc.Committed.Number(), "hash", bc.Committed.Hash())
		m, err := e2c.Encode(c.committed)
		if err != nil {
			return err
		}
		mm := &e2c.Message{
			Code:    e2c.VoteMsgCode,
			Msg:     m,
			Address: c.backend.Address(),
		}
		payload, err := mm.PayloadWithSig(c.backend.Sign)
		if err != nil {
			return err
		}
		c.backend.SendToOne(payload, msg.Address)
	}
	if bc.Lock.Number().Uint64() >= c.committed.Number().Uint64() && bc.Lock.Number().Uint64() <= c.lock.Number().Uint64() {
		c.logger.Info("Voted for block", "number", bc.Lock.Number(), "hash", bc.Lock.Hash())
		m, err := e2c.Encode(c.lock)
		if err != nil {
			return err
		}
		mm := &e2c.Message{
			Code:    e2c.VoteMsgCode,
			Msg:     m,
			Address: c.backend.Address(),
		}
		payload, err := mm.PayloadWithSig(c.backend.Sign)
		if err != nil {
			return err
		}
		c.backend.SendToOne(payload, msg.Address)
	}
	return nil
}

func (c *core) handleValidate(msg *e2c.Message) error {
	var t time.Time
	if err := msg.Decode(&t); err != nil {
		return err
	}

	c.logger.Info("Received validate message", "addr", msg.Address)
	c.validates[msg.Address] = msg
	// @todo replace with F, but can't now because node 1 dies when it's bad
	if uint64(len(c.validates)) == 2 {
		block := <-c.ch

		var validates []*e2c.Message
		for _, val := range c.validates {
			validates = append(validates, val)
		}

		c.backend.SendFinal(e2c.B2{Block: block, Validates: validates})
		c.logger.Info("Sent proposal for second block in view", "number", block.Number(), "hash", block.Hash(), "validates", validates)
		c.backend.SetStatus(0)
	}
	return nil
}

func (c *core) handleVote(msg *e2c.Message) error {
	var block *types.Block
	if err := msg.Decode(&block); err != nil {
		return err
	}
	// @todo don't recalculate hash
	c.vote[block.Hash()][msg.Address] = msg
	c.logger.Info("Received vote message", "number", block.Number(), "hash", block.Hash(), "addr", msg.Address)
	if uint64(len(c.vote[block.Hash()])) > c.config.F {

		if c.highestCert == nil {
			var votes []*e2c.Message
			for _, val := range c.vote[block.Hash()] {
				votes = append(votes, val)
			}

			c.highestCert = &e2c.BlockCertificate{
				Block: block,
				Votes: votes,
			}

			if c.backend.Address() == c.backend.Leader() {
				c.logger.Info("Block is certified!", "number", block.Number(), "hash", block.Hash(), "votes", votes)
			} else {
				c.logger.Info("Block is certified! Sending to leader", "number", block.Number(), "hash", block.Hash(), "votes", votes)
			}

			m, err := e2c.Encode(c.highestCert)
			if err != nil {
				return err
			}
			mm := &e2c.Message{
				Code:    e2c.BlockCertMsgCode,
				Msg:     m,
				Address: c.backend.Address(),
			}
			payload, err := mm.PayloadWithSig(c.backend.Sign)
			if err != nil {
				return err
			}
			c.backend.SendToOne(payload, c.backend.Leader())

			var block *e2c.BlockCertificate
			if err := mm.Decode(&block); err != nil {
				return err
			}

		} else if c.highestCert.Block.Number().Uint64() < block.Number().Uint64() {

			var votes []*e2c.Message
			for _, val := range c.vote[block.Hash()] {
				votes = append(votes, val)
			}

			c.highestCert = &e2c.BlockCertificate{
				Block: block,
				Votes: votes,
			}

			if c.backend.Address() == c.backend.Leader() {
				c.logger.Info("Block is certified!", "number", block.Number(), "hash", block.Hash(), "votes", votes)
			} else {
				c.logger.Info("Block is certified! Sending to leader", "number", block.Number(), "hash", block.Hash(), "votes", votes)
			}
			m, err := e2c.Encode(c.highestCert)
			if err != nil {
				return err
			}
			mm := &e2c.Message{
				Code:    e2c.BlockCertMsgCode,
				Msg:     m,
				Address: c.backend.Address(),
			}
			payload, err := mm.PayloadWithSig(c.backend.Sign)
			if err != nil {
				return err
			}
			c.backend.SendToOne(payload, c.backend.Leader())
		}
	}
	return nil
}

// @todo reject this message if you aren't the leader
func (c *core) handleBlockCert(msg *e2c.Message) error {
	var block *e2c.BlockCertificate
	if err := msg.Decode(&block); err != nil {
		return err
	}

	c.logger.Info("Block certificate received!", "addr", msg.Address)

	if uint64(len(block.Votes)) <= c.config.F {
		c.logger.Warn("Block certificate rejected", "err", "not enough votes")
		return nil
	}
	for _, m := range block.Votes {
		if err := m.VerifySig(); err != nil || m.Code != e2c.VoteMsgCode {
			c.logger.Warn("Block certificate rejected", "err", "vote message invalid")
			return nil
		}
	}

	if c.highestCert == nil {
		c.highestCert = block
	} else if c.highestCert.Block.Number().Uint64() < block.Block.Number().Uint64() {
		c.highestCert = block
	}

	return nil
}

func (c *core) sendB1() {

	for {
		block, ok := c.blockQueue.getNext()
		if !ok {
			break
		}

		if block.Number().Uint64() <= c.highestCert.Block.Number().Uint64() {
			c.commit(block)
		}
	}

	c.backend.SetStatus(2)
	block := <-c.ch
	c.backend.SetStatus(3)
	c.logger.Info("Proposing new block", "number", block.Number(), "hash", block.Hash().String(), "certificate", c.highestCert)
	c.blockQueue.clear()
	c.backend.SendBlockOne(e2c.B1{Cert: c.highestCert, Block: block})
}

func (c *core) handleB1(msg *e2c.Message) error {
	var b e2c.B1
	if err := msg.Decode(&b); err != nil {
		return err
	}

	c.logger.Info("Proposal for first block in view received", "number", b.Block.Number(), "hash", b.Block.Hash().String())

	if b.Cert.Block.Number().Uint64() < c.highestCert.Block.Number().Uint64() {
		c.backend.SendBlame()
		c.logger.Warn("Blame sent", "err", "does not extend block")
		return nil
	}

	if uint64(len(b.Cert.Votes)) <= c.config.F {
		c.backend.SendBlame()
		c.logger.Warn("Blame sent", "err", "does not contain enough votes")
		return nil
	}
	for _, m := range b.Cert.Votes {
		if err := m.VerifySig(); err != nil || m.Code != e2c.VoteMsgCode {
			c.backend.SendBlame()
			c.logger.Warn("Blame sent", "err", "votes not valid")
			return nil
		}
	}

	for {
		block, ok := c.blockQueue.getNext()
		if !ok {
			break
		}

		if block.Number().Uint64() <= c.highestCert.Block.Number().Uint64() {
			c.commit(block)
		}
	}
	c.blockQueue.clear()
	if err := c.verify(b.Block); err != nil {
		c.logger.Warn("Blame sent", "err", "invalid block")
		c.backend.SendBlame()
	}
	c.blockQueue.insertHandled(b.Block)
	c.lock = b.Block
	c.backend.SendValidate()
	c.backend.SetStatus(2)
	c.logger.Info("Sending validate for first block in view", "number", b.Block.Number(), "hash", b.Block.Hash().String())
	return nil
}

func (c *core) handleB2(msg *e2c.Message) error {
	var b e2c.B2
	if err := msg.Decode(&b); err != nil {
		return err
	}

	c.logger.Info("Proposal for second block in view received", "number", b.Block.Number(), "hash", b.Block.Hash().String(), "validates", b.Validates)

	// @todo check that each message if from different addr
	// @todo change 2 to F when we get it working
	if uint64(len(b.Validates)) < 2 {
		c.backend.SendBlame()
		c.logger.Warn("Blame sent", "err", "does not contain enough validates")
		return nil
	}
	for _, m := range b.Validates {
		if err := m.VerifySig(); err != nil || m.Code != e2c.ValidateMsgCode {
			c.backend.SendBlame()
			c.logger.Warn("Blame sent", "err", "validates not valid")
			return nil
		}
	}

	if err := c.verify(b.Block); err != nil {
		c.logger.Warn("Blame sent", "err", "invalid block")
		c.backend.SendBlame()
	}
	c.blockQueue.insertHandled(b.Block)
	c.lock = b.Block
	c.backend.SetStatus(0)
	c.logger.Info("View Change completed! Resuming normal operations")
	return nil
}
