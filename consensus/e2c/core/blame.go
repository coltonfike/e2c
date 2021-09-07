package core

import (
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
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

// if we blame due to equivocation we need to send equivocating blocks
func (c *core) sendEquivBlame(b1 *types.Block, b2 *types.Block) error {

	// equivBlame message inclues a normal blame message plus the equivocating blocks
	// this is the normal blame message
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

// send the blame certificate to notify all nodes to quit the view
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

	log.Info("Blame message received", "addr", msg.Address, "total blame", len(c.blame))
	c.checkBlame()
	return true
}

// handle equivblame message
func (c *core) handleEquivBlame(msg *Message) bool {

	var blame EquivBlame
	if err := msg.Decode(&blame); err != nil {
		log.Error("Failed to decode blame message", "err", err)
		return false
	}

	// ensure that the blocks included do actually equivocate
	if blame.B1.Number().Uint64() == blame.B2.Number().Uint64() && blame.B1.Hash() != blame.B2.Hash() && c.backend.IsSignerLeader(blame.B1) && c.backend.IsSignerLeader(blame.B2) {
		return c.handleBlameMessage(blame.Blame)
	}
	return false
}

// everytime we send blame, we need to check how much total blame has been
// received so we know if we need to view change
func (c *core) checkBlame() {

	// see if we have enough blame messages to change view
	if uint64(len(c.blame)) == c.backend.F()+1 {

		if err := c.sendBlameCertificate(); err != nil {
			log.Error("Failed to send blame certificate", "err", err)
		}

		// quit the view on the backend and then wait for all other nodes to quit
		c.backend.ChangeView()
		// wait 1 delta for all nodes to quit view
		<-time.After(c.config.Delta * time.Millisecond)
		// start the view change protocol
		c.changeView()
	}
}

// handles a blame certificate
func (c *core) handleBlameCertificate(msg *Message) bool {

	var blames [][]byte
	if err := msg.Decode(&blames); err != nil {
		log.Error("Failed to decode blame message", "err", err)
		return false
	}

	// verify that all the blame messages included are valid
	if uint64(len(blames)) <= c.backend.F() {
		log.Warn("Not enough blames")
		return false
	}

	// in order for us to check that signatures are valid, we make a dummy message that
	// the signatures should have signed
	m, err := Encode(blames)
	if err != nil {
		log.Error("Failed to encode message", "err", err)
		return false
	}
	ms := &Message{
		Code: BlameMsg,
		Msg:  m,
		View: c.backend.View(),
	}
	// verify signatures are correct on the dummy message
	if err := VerifyCertificateSignatures(ms, blames, c.checkValidatorSignature); err != nil {
		log.Error("Invalid signature on blame message", "err", err)
		return false
	}

	c.backend.ChangeView()
	<-time.After(c.config.Delta * time.Millisecond)
	c.changeView()
	return true
}
