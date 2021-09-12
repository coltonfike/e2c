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
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/e2c"
	"github.com/ethereum/go-ethereum/p2p"
	lru "github.com/hashicorp/golang-lru"
)

const (
	e2cMsg            = 0x11
	NewBlockMsg       = 0x07
	NewBlockHashesMsg = 0x01
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
		if b.coreStarted {
			return true, nil
		}
		// client nodes do not handle this message. eth.Handler should call the client handling method while handling this
		return false, nil
	}
	// Likewise, we reject block header announcements sent by fetcher
	// client also rejects these. This means clients can't accept block headers, only full blocks
	if msg.Code == NewBlockHashesMsg {
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
func (b *backend) NewChainHead() error {
	return nil
}
