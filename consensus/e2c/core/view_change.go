// This implements the code needed for the view change protocol
package core

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/core/types"
)

func (c *core) handleBlameMessage(msg *e2c.Message) error {
	var t time.Time
	if err := msg.Decode(&t); err != nil {
		fmt.Println("DDDD")
		return err
	}

	// @todo maybe remove this?
	if c.backend.Address() == c.backend.Leader() {
		return nil
	}

	c.blame[msg.Address] = msg // add this message to our blame map

	c.logger.Info("Blame message received", "total blame", len(c.blame))

	// @todo send blame certificate, then wait 1 delta, then send votes..
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
		fmt.Println("Waiting!!")
		<-time.After(c.config.Delta * time.Millisecond)
		fmt.Println("Done waiting!!")

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

		// @todo don't recalculate hash each time here
		fmt.Println("Hashes", c.committed.Hash().String(), c.lock.Hash().String())
		c.vote[c.committed.Hash()] = make(map[common.Address]*e2c.Message)
		c.vote[c.lock.Hash()] = make(map[common.Address]*e2c.Message)
		c.vote[c.committed.Hash()][c.backend.Address()] = v1
		c.vote[c.lock.Hash()][c.backend.Address()] = v2
		c.backend.SendBlameCertificate(e2c.BlameCertificate{Lock: c.lock, Committed: c.committed})
		if c.backend.Address() == c.backend.Leader() {
			fmt.Println("\n\n\nResetting Timer")
			c.certTimer.Reset(4 * c.config.Delta * time.Millisecond)
		}
	}
	return nil
}

func (c *core) handleCert(msg *e2c.Message) error {
	var bc e2c.BlameCertificate
	if err := msg.Decode(&bc); err != nil {
		fmt.Println("EEE")
		return err
	}
	fmt.Println("Handling Cert!")
	if bc.Committed.Number().Uint64() >= c.committed.Number().Uint64() && bc.Committed.Number().Uint64() <= c.lock.Number().Uint64() {
		fmt.Println("Sent vote")
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
		fmt.Println("Sent vote")
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
		fmt.Println("FFF")
		return err
	}

	fmt.Println("Received validate")
	c.validates[msg.Address] = msg
	// @todo replace with F, but can't now because node 1 dies when it's bad
	if uint64(len(c.validates)) >= 2 {
		fmt.Println("Trying to read from ch")
		block := <-c.ch

		var validates []*e2c.Message
		for _, val := range c.validates {
			validates = append(validates, val)
		}

		c.backend.SendFinal(e2c.B2{Block: block, Validates: validates})
		fmt.Println("Sent final")
		c.backend.SetStatus(0)
	}
	return nil
}

func (c *core) handleVote(msg *e2c.Message) error {
	var block *types.Block
	if err := msg.Decode(&block); err != nil {
		fmt.Println("GGG")
		return err
	}
	// @todo don't recalculate hash
	c.vote[block.Hash()][msg.Address] = msg
	fmt.Println("Vote received!!")
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

			fmt.Println(c.highestCert.Votes)
			fmt.Println("Block", block.Number().Uint64(), "certified. Sending certificate to new leader!")

			m, err := e2c.Encode(c.highestCert)
			if err != nil {
				fmt.Println("A", err)
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
				fmt.Println("Here", err)
				return err
			}
			fmt.Println("\n\nDecoded success!!")

		} else if c.highestCert.Block.Number().Uint64() < block.Number().Uint64() {

			var votes []*e2c.Message
			for _, val := range c.vote[block.Hash()] {
				votes = append(votes, val)
			}

			c.highestCert = &e2c.BlockCertificate{
				Block: block,
				Votes: votes,
			}
			fmt.Println("Block", block.Number().Uint64(), "certified. Sending certificate to new leader!")
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

func (c *core) handleBlockCert(msg *e2c.Message) error {
	var block *e2c.BlockCertificate
	if err := msg.Decode(&block); err != nil {
		fmt.Println("here", err)
		return err
	}

	c.certReceived++
	fmt.Println("\n\nBlockCert Received!!")
	fmt.Println(block.Votes)

	if uint64(len(block.Votes)) <= c.config.F {
		fmt.Println("Block cert not enough votes")
		return nil
	}
	for _, m := range block.Votes {
		if err := m.VerifySig(); err != nil || m.Code != e2c.VoteMsgCode {
			fmt.Println("block cert not being valid")
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

	fmt.Println("Highest is", c.highestCert.Block.Number())

	for {
		block, ok := c.blockQueue.getNext()
		if !ok {
			break
		}
		fmt.Println(block.Number())

		if block.Number().Uint64() <= c.highestCert.Block.Number().Uint64() {
			c.commit(block)
		}
	}

	c.backend.SetStatus(2)
	block := <-c.ch
	c.backend.SetStatus(3)
	fmt.Println("Block", block.Number())
	c.blockQueue.clear()
	c.backend.SendBlockOne(e2c.B1{Cert: c.highestCert, Block: block})
}

func (c *core) handleB1(msg *e2c.Message) error {
	var b e2c.B1
	if err := msg.Decode(&b); err != nil {
		fmt.Println("HHH")
		return err
	}
	if b.Cert.Block.Number().Uint64() < c.highestCert.Block.Number().Uint64() {
		c.backend.SendBlame()
		fmt.Println("Blame Sent for not extending block")
		return nil
	}

	if uint64(len(b.Cert.Votes)) <= c.config.F {
		c.backend.SendBlame()
		fmt.Println("Blame Sent for not enough votes")
		return nil
	}
	for _, m := range b.Cert.Votes {
		if err := m.VerifySig(); err != nil || m.Code != e2c.VoteMsgCode {
			c.backend.SendBlame()
			fmt.Println("Blame sent for block cert not being valid")
			return nil
		}
	}

	for {
		block, ok := c.blockQueue.getNext()
		if !ok {
			break
		}
		fmt.Println(block.Number())

		if block.Number().Uint64() <= c.highestCert.Block.Number().Uint64() {
			c.commit(block)
		}
	}
	c.blockQueue.clear()
	if err := c.verify(b.Block); err != nil {
		c.backend.SendBlame()
	}
	c.blockQueue.insertHandled(b.Block)
	c.lock = b.Block
	c.backend.SendValidate()
	c.backend.SetStatus(2)
	fmt.Println("Check 1 Passed, sending Ack")
	return nil
}

func (c *core) handleB2(msg *e2c.Message) error {
	var b e2c.B2
	if err := msg.Decode(&b); err != nil {
		fmt.Println("CCC", err)
		return err
	}

	// @todo do error checking here
	// @todo check that each message if from different addr
	// @todo change 2 to F when we get it working
	if uint64(len(b.Validates)) < 2 {
		c.backend.SendBlame()
		fmt.Println("Blame Sent for not enough Validates")
		return nil
	}
	for _, m := range b.Validates {
		if err := m.VerifySig(); err != nil || m.Code != e2c.ValidateMsgCode {
			c.backend.SendBlame()
			fmt.Println("Blame sent for validate msg not being valid")
			return nil
		}
	}

	fmt.Println("Received final block!")
	if err := c.verify(b.Block); err != nil {
		c.backend.SendBlame()
	}
	c.blockQueue.insertHandled(b.Block)
	c.lock = b.Block
	c.backend.SetStatus(0)
	return nil
}
