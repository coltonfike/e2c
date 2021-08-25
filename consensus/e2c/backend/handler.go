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
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/p2p"
	lru "github.com/hashicorp/golang-lru"
)

const (
	e2cMsg            = 0x11
	NewBlockMsg       = 0x07
	NewBlockHashesMsg = 0x01
)

var (
	// errDecodeFailed is returned when decode message fails
	errDecodeFailed = errors.New("fail to decode e2c message")
)

// Protocol implements consensus.Engine.Protocol
func (b *backend) Protocol() consensus.Protocol {
	return consensus.IstanbulProtocol // E2C runs on the IstanbulProtocol
}

func (b *backend) decode(msg p2p.Msg) ([]byte, common.Hash, error) {
	var data []byte
	if err := msg.Decode(&data); err != nil {
		return nil, common.Hash{}, errDecodeFailed
	}

	return data, e2c.RLPHash(data), nil
}

// HandleMsg implements consensus.Handler.HandleMsg
func (b *backend) HandleMsg(addr common.Address, msg p2p.Msg) (bool, error) {
	b.coreMu.Lock()
	defer b.coreMu.Unlock()
	if msg.Code == e2cMsg && b.coreStarted {

		data, hash, err := b.decode(msg)
		if err != nil {
			return true, errDecodeFailed
		}
		// Mark peer's message
		ms, ok := b.recentMessages.Get(addr)
		var m *lru.ARCCache
		if ok {
			m, _ = ms.(*lru.ARCCache)
		} else {
			m, _ = lru.NewARC(inmemoryMessages)
			b.recentMessages.Add(addr, m)
		}
		m.Add(hash, true)

		// Mark self known message
		if _, ok := b.knownMessages.Get(hash); ok {
			return true, nil
		}
		b.knownMessages.Add(hash, true)

		// Send the message to the e2c.Core for handling
		go b.eventMux.Post(e2c.MessageEvent{Payload: data})
	}
	// We commit our own blocks, and thus, don't want this to run on the protocol manager
	if msg.Code == NewBlockMsg {
		// ignore if we aren't a client node, since we receive our blocks in the above if
		if b.coreStarted {
			return true, nil
		}

		var request struct {
			Block *types.Block
			TD    *big.Int
		}
		if err := msg.Decode(&request); err != nil {
			return true, err
		}

		// add to the count of how many times we have seen this block
		if n, ok := b.clientBlocks[request.Block.Hash()]; ok {
			b.clientBlocks[request.Block.Hash()] = n + 1
		} else {
			b.clientBlocks[request.Block.Hash()] = 1
		}

		b.logger.Info("[E2C] Block acknowledgement received", "number", request.Block.Number(), "hash", request.Block.Hash(), "total acks", b.clientBlocks[request.Block.Hash()])
		// @todo we would like to use b.F() and to check these messages came from out validator set, but I don't know how to get that information
		// to the client. This code doesn't get access to the chain, only validator nodes get that, and because we don't have access to the chain
		// we can't get access to the genesis block which has the list of validators. I'm sure there is a way to get that information to the client
		// but I have looked for about 5 hours and can't find it, so I'm letting a network weakness exist for clients and using an extra field in the config
		// to work around it until I find a solution
		if b.clientBlocks[request.Block.Hash()] == b.config.F+1 {
			b.Commit(request.Block)

			// Delete the parent block. If we delete the one we just committed
			// then we will probably see it again since we commit at F+1 acks
			// This means it doesn't actually get delete and we will run into
			// Memory errors. So instead we delete the parent since we shouldn't
			// See that block anymore. We could add a timeout function to delete
			// after a set interval
			delete(b.clientBlocks, request.Block.ParentHash())
			b.logger.Info("[E2C] Client committed block", "number", request.Block.Number(), "hash", request.Block.Hash())
		}
		return true, nil
	}
	// Likewise, we reject block header announcements sent by fetcher
	if msg.Code == NewBlockHashesMsg {
		if b.coreStarted {
			return true, nil
		}
		// @todo Maybe the client should handle the block headers?
		return true, nil
	}
	return false, nil
}

// SetBroadcaster implements consensus.Handler.SetBroadcaster
func (b *backend) SetBroadcaster(broadcaster consensus.Broadcaster) {
	b.broadcaster = broadcaster
}

// NewChainHead implements consensus.Handler.SetBroadcaster
// It's useless in E2C, we just have it so we can use the consensus.Handler
// like in Istanbul
func (b *backend) NewChainHead() error {
	return nil
}
