package core

import (
	"time"

	"github.com/ethereum/go-ethereum/consensus/e2c"
)

func (c *core) sendBlame() error {

	if c.backend.Address() == c.backend.Leader() {
		return nil
	}

	c.blame[c.backend.Address()] = &Message{}

	// @todo send equivocating blocks?

	c.broadcast(&Message{
		Code: BlameMsg,
	})
	return nil
}

func (c *core) sendBlameCertificate() error {

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

func (c *core) handleBlameMessage(msg *Message) bool {

	if msg.View != c.backend.View() { // blame for different view
		return false
	}

	// @todo do checking of blocks on equivocation if necessary?

	c.blame[msg.Address] = msg // add this message to our blame map

	c.logger.Info("[E2C] Blame message received", "addr", msg.Address, "total blame", len(c.blame))

	if uint64(len(c.blame)) > c.config.F {
		if c.backend.Status() != e2c.SteadyState { // we are already changing view
			return false
		}
		if err := c.sendBlameCertificate(); err != nil {
			c.logger.Error("Failed to send blame certificate", "err", err)
		}
		// @todo maybe change this to quit view? we need to stop before accepting more messages
		c.backend.ChangeView()
		<-time.After(c.config.Delta * time.Millisecond) // wait 1 delta for all nodes to quit view
		c.changeView()
	}
	return true
}

func (c *core) handleBlameCertificate(msg *Message) bool {

	if c.backend.Status() != e2c.SteadyState { // we are already changing view
		return false
	}

	var cert BlameCert
	if err := msg.Decode(&cert); err != nil {
		c.logger.Error("Failed to decode blame message", "err", err)
		return false
	}

	for _, msg := range cert.Blames {
		var blame Blame
		if err := msg.Decode(&msg); err != nil {
			c.logger.Error("Invalid blame message contained in certificate", "err", err)
			return false
		}
		if blame.View != c.backend.View() { // blame for different view
			return false
		}
		// @todo do checking of blocks on equivocation if necessary?
	}
	c.backend.ChangeView()
	<-time.After(c.config.Delta * time.Millisecond) // Maybe make this include a time stamp and wait the remaining time?
	c.changeView()
	return true
}
