package core

import (
	"time"

	"github.com/ethereum/go-ethereum/core/types"
)

// this sends a blame message to all nodes
func (c *core) sendBlame() error {

	// don't let leader blame itself. this can happen when progress timer expires
	if c.backend.Address() == c.backend.Leader() {
		return nil
	}

	msg := &Message{
		Code: BlameMsg,
	}

	c.broadcast(msg)

	c.blame[c.backend.Address()] = msg.Signature
	c.checkBlame()
	return nil
}

func (c *core) sendEquivBlame(b1 *types.Block, b2 *types.Block) error {

	msg := &Message{
		Code:    BlameMsg,
		View:    c.backend.View(),
		Address: c.backend.Address(),
	}
	msg.Sign(c.backend.Sign)

	m, err := Encode(&EquivBlame{
		Blame: msg,
		B1:    b1,
		B2:    b2,
	})
	if err != nil {
		return err
	}
	mm := &Message{
		Code: EquivBlameMsg,
		Msg:  m,
	}

	c.broadcast(mm)

	c.blame[c.backend.Address()] = msg.Signature
	c.checkBlame()
	return nil
}

// send the blame certificate
func (c *core) sendBlameCertificate() error {

	// append all the blame messages received to the certificate
	var blames [][]byte
	for _, m := range c.blame {
		blames = append(blames, m)
	}

	msg, err := Encode(blames)
	if err != nil {
		return err
	}

	c.broadcast(&Message{
		Code: BlameCertificateMsg,
		Msg:  msg,
	})
	return nil
}

// handle a blame message
func (c *core) handleBlameMessage(msg *Message) bool {

	c.blame[msg.Address] = msg.Signature // add this message to our blame map

	c.logger.Info("[E2C] Blame message received", "addr", msg.Address, "total blame", len(c.blame))
	c.checkBlame()
	return true
}

func (c *core) handleEquivBlame(msg *Message) bool {

	var blame EquivBlame
	if err := msg.Decode(&blame); err != nil {
		c.logger.Error("Failed to decode blame message", "err", err)
		return false
	}

	if blame.B1.Number().Uint64() == blame.B2.Number().Uint64() && blame.B1.Hash() != blame.B2.Hash() && c.backend.IsSignerLeader(blame.B1) && c.backend.IsSignerLeader(blame.B2) {
		return c.handleBlameMessage(blame.Blame)
	}
	return false
}

func (c *core) checkBlame() {

	// see if we have enough blame messages to change view
	if uint64(len(c.blame)) == c.backend.F()+1 {

		if err := c.sendBlameCertificate(); err != nil {
			c.logger.Error("Failed to send blame certificate", "err", err)
		}

		// quit the view on the backend and then wait for all other nodes to quit
		c.backend.ChangeView()
		<-time.After(c.config.Delta * time.Millisecond) // wait 1 delta for all nodes to quit view
		// start the view change protocol
		c.changeView()
	}
}

// handles a blame certificate
func (c *core) handleBlameCertificate(msg *Message) bool {

	var blames [][]byte
	if err := msg.Decode(&blames); err != nil {
		c.logger.Error("Failed to decode blame message", "err", err)
		return false
	}

	// verify that all the blame messages included are valid
	if uint64(len(blames)) <= c.backend.F() {
		c.logger.Error("Not enough blames")
		return false
	}

	m, err := Encode(blames)
	if err != nil {
		c.logger.Error("Failed to encode message", "err", err)
		return false
	}
	ms := &Message{
		Code: BlameMsg,
		Msg:  m,
		View: c.backend.View(),
	}

	if err := VerifyCertificateSignatures(ms, blames, c.checkValidatorSignature); err != nil {
		c.logger.Error("Invalid signature on blame message", "err", err)
		return false
	}

	c.backend.ChangeView()
	<-time.After(c.config.Delta * time.Millisecond) // @todo Maybe make this include a time stamp and wait the remaining time?
	c.changeView()
	return true
}
