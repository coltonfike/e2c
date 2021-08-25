package core

import (
	"time"

	"github.com/ethereum/go-ethereum/consensus/e2c"
)

// this sends a blame message to all nodes
func (c *core) sendBlame() error {

	// don't let leader blame itself. this can happen when progress timer expires
	if c.backend.Address() == c.backend.Leader() {
		return nil
	}

	c.blame[c.backend.Address()] = &Message{}

	// @todo send equivocating blocks??

	c.broadcast(&Message{
		Code: BlameMsg,
	})
	return nil
}

// send the blame certificate
func (c *core) sendBlameCertificate() error {

	// append all the blame messages received to the certificate
	var blames []*Message
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

	if msg.View != c.backend.View() { // blame for different view
		return false
	}

	// @todo do checking of blocks on equivocation if necessary?

	c.blame[msg.Address] = msg // add this message to our blame map

	c.logger.Info("[E2C] Blame message received", "addr", msg.Address, "total blame", len(c.blame))

	// see if we have enough blame messages to change view
	if uint64(len(c.blame)) == c.backend.F()+1 {

		// check to see if we are already changing view
		if c.backend.Status() != e2c.SteadyState {
			return false
		}

		if err := c.sendBlameCertificate(); err != nil {
			c.logger.Error("Failed to send blame certificate", "err", err)
		}

		// quit the view on the backend and then wait for all other nodes to quit
		c.backend.ChangeView()
		<-time.After(c.config.Delta * time.Millisecond) // wait 1 delta for all nodes to quit view
		// start the view change protocol
		c.changeView()

	}
	return true
}

// handles a blame certificate
func (c *core) handleBlameCertificate(msg *Message) bool {

	if c.backend.Status() != e2c.SteadyState { // we are already changing view
		return false
	}

	var cert BlameCert
	if err := msg.Decode(&cert); err != nil {
		c.logger.Error("Failed to decode blame message", "err", err)
		return false
	}

	// verify that all the blame messages included are valid
	if uint64(len(cert.Blames)) <= c.backend.F() {
		c.logger.Error("Not enough blames")
		return false
	}
	for _, msg := range cert.Blames {
		if err := msg.VerifySig(c.checkValidatorSignature); err != nil || msg.Code != BlameMsg {
			c.logger.Error("Invalid signature on blame message", "err", err)
		}
		// @todo do checking of blocks on equivocation if necessary?
	}

	c.backend.ChangeView()
	<-time.After(c.config.Delta * time.Millisecond) // @todo Maybe make this include a time stamp and wait the remaining time?
	c.changeView()
	return true
}
