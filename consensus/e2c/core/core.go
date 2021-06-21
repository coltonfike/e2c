package core

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

type Core struct {
	e2c           e2c.Engine
	commitTimer   *time.Timer // alert when a blocks timer expires
	progressTimer *e2c.ProgressTimer
	nextBlock     common.Hash              // Next block whose timer will go off
	queuedBlocks  map[common.Hash]struct { // this is the queue of all blocks not yet committed
		block *types.Block
		time  time.Time
	}
	eventMux       *event.TypeMuxSubscription
	handlerwg      *sync.WaitGroup
	delta          time.Duration
	expectedHeight *big.Int
}

func New(e2c e2c.Engine, delta int) *Core {
	core := &Core{
		e2c: e2c,
		queuedBlocks: make(map[common.Hash]struct {
			block *types.Block
			time  time.Time
		}),
		handlerwg:      new(sync.WaitGroup),
		delta:          time.Duration(delta),
		expectedHeight: big.NewInt(1), // TODO: Make this look for the block at the top of the chain!!!!
	}
	core.Start()
	return core
}

func (c *Core) Start() {
	c.commitTimer = time.NewTimer(time.Millisecond)
	c.progressTimer = e2c.NewProgressTimer(4 * c.delta * time.Second)
	c.subscribeEvents()
	go c.loop()
}

func (c *Core) subscribeEvents() {
	c.eventMux = c.e2c.EventMux().Subscribe(e2c.BlockProposal{}, e2c.Ack{})
}

func (c *Core) unsubscribeEvents() {
	c.eventMux.Unsubscribe()
}

func (c *Core) Stop() error {
	c.unsubscribeEvents()
	c.handlerwg.Wait()
	return nil
}

func (c *Core) resetTimer() error {
	earliestTime := time.Now()
	var earliestBlock common.Hash

	for block, t := range c.queuedBlocks {
		if t.time.Before(earliestTime) {
			earliestTime = t.time
			earliestBlock = block
		}
	}

	d := time.Until(earliestTime.Add(2 * c.delta * time.Second))
	if d <= 0 {
		return errors.New("Timer already expired")
	}

	c.commitTimer.Reset(d)
	c.nextBlock = earliestBlock

	return nil
}

func (c *Core) verify(block *types.Block) error {

	if err := c.e2c.Verify(block.Header()); err != nil {
		if err != consensus.ErrUnknownAncestor {
			return err
		}

		parent, exists := c.queuedBlocks[block.ParentHash()]
		if !exists {
			fmt.Println("Blocks Arrived Out of Order")
			return nil // TODO: Return Error
		}
		// check this again, as it hasn't been checked due to there not being a parent
		if parent.block.Number().Uint64()+1 != block.Number().Uint64() {
			return err
		}
		// All ok, check equivocation
	}
	if block.Number().Uint64() != c.expectedHeight.Uint64() {
		return errors.New("Already received block at this height")
	}
	return nil
}

func (c *Core) delete(block common.Hash) {
	delete(c.queuedBlocks, block)
}
