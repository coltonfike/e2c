// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package backend

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	e2cCore "github.com/ethereum/go-ethereum/consensus/e2c/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	lru "github.com/hashicorp/golang-lru"
)

const (
	// fetcherID is the ID indicates the block is from Istanbul engine
	fetcherID = "e2c"
)

// New creates an Ethereum backend for Istanbul core engine.
func New(config *e2c.Config, privateKey *ecdsa.PrivateKey, db ethdb.Database) consensus.Istanbul {
	// Allocate the snapshot caches and create the engine
	recents, _ := lru.NewARC(inmemorySnapshots)
	recentMessages, _ := lru.NewARC(inmemoryPeers)
	knownMessages, _ := lru.NewARC(inmemoryMessages)
	backend := &backend{
		config:         config,
		eventMux:       new(event.TypeMux),
		privateKey:     privateKey,
		address:        crypto.PubkeyToAddress(privateKey.PublicKey),
		logger:         log.New(),
		db:             db,
		recents:        recents,
		coreStarted:    false,
		recentMessages: recentMessages,
		knownMessages:  knownMessages,
	}
	backend.core = e2cCore.New(backend, backend.config)
	return backend
}

// ----------------------------------------------------------------------------

type backend struct {
	config      *e2c.Config
	eventMux    *event.TypeMux
	privateKey  *ecdsa.PrivateKey
	address     common.Address
	core        e2c.Engine
	logger      log.Logger
	db          ethdb.Database
	chain       consensus.ChainHeaderReader
	sealMu      sync.Mutex
	coreStarted bool
	coreMu      sync.RWMutex

	// Snapshots for recent block to speed up reorgs
	recents *lru.ARCCache

	// event subscription for ChainHeadEvent event
	broadcaster consensus.Broadcaster

	recentMessages *lru.ARCCache // the cache of peer's messages
	knownMessages  *lru.ARCCache // the cache of self messages
}

func (b *backend) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	return new(big.Int)
}

// Address implements e2c.Backend.Address
func (b *backend) Address() common.Address {
	return b.address
}

// Validators implements e2c.Backend.Validators
func (b *backend) Leader(block *types.Block) common.Address {
	snap, err := b.snapshot(b.chain, block.Number().Uint64(), block.Hash(), nil)
	if err != nil {
		return common.Address{}
	}
	return snap.Leader
}

// Broadcast implements e2c.Backend.Gossip
func (b *backend) Broadcast(payload []byte) error {
	hash := e2c.RLPHash(payload)
	b.knownMessages.Add(hash, true)

	if b.broadcaster != nil {
		ps := b.broadcaster.PeerSet()
		fmt.Println("PeerSet:", len(ps))
		for addr, p := range ps {
			ms, ok := b.recentMessages.Get(addr)
			var m *lru.ARCCache
			if ok {
				m, _ = ms.(*lru.ARCCache)
				if _, k := m.Get(hash); k {
					// This peer had this event, skip it
					continue
				}
			} else {
				m, _ = lru.NewARC(inmemoryMessages)
			}

			m.Add(hash, true)
			b.recentMessages.Add(addr, m)
			go p.SendConsensus(e2cMsg, payload)
		}
	}
	return nil
}

func (b *backend) SendNewBlock(block *types.Block) error {

	msg, err := Encode(&e2c.NewBlockEvent{Block: block})
	if err != nil {
		return err
	}
	m := &message{
		Code:    newBlockMsgCode,
		Msg:     msg,
		Address: b.address,
	}
	payload, err := m.PayloadWithSig(b.Sign)
	if err != nil {
		return err
	}
	go b.Broadcast(payload)
	return nil
}

func (b *backend) RelayBlock(header *types.Header) error {

	msg, err := Encode(&e2c.RelayBlockEvent{Header: header})
	if err != nil {
		return err
	}
	m := &message{
		Code:    relayMsgCode,
		Msg:     msg,
		Address: b.address,
	}

	payload, err := m.PayloadWithSig(b.Sign)
	if err != nil {
		return err
	}
	go b.Broadcast(payload)
	return nil
}

func (b *backend) SendBlame(addr common.Address) error {

	msg, err := Encode(&e2c.BlameEvent{Address: addr})
	if err != nil {
		return err
	}
	m := &message{
		Code:    blameMsgCode,
		Msg:     msg,
		Address: b.address,
	}
	payload, err := m.PayloadWithSig(b.Sign)
	if err != nil {
		return err
	}
	go b.Broadcast(payload)
	return nil
}

// Commit implements e2c.Backend.Commit
func (b *backend) Commit(block *types.Block) error {
	b.broadcaster.Enqueue(fetcherID, block)
	return nil
}

// EventMux implements e2c.Backend.EventMux
func (b *backend) EventMux() *event.TypeMux {
	return b.eventMux
}

// Verify implements e2c.Backend.Verify
func (b *backend) Verify(block *types.Block) error {
	return nil
}

// Sign implements e2c.Backend.Sign
func (b *backend) Sign(data []byte) ([]byte, error) {
	hashData := crypto.Keccak256(data)
	return crypto.Sign(hashData, b.privateKey)
}

func (b *backend) Close() error {
	return nil
}
