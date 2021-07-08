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

package e2c

import (
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

type NewBlockEvent struct {
	Block *types.Block
}

// EncodeRLP serializes b into the Ethereum RLP format.
func (e *NewBlockEvent) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{e.Block})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (e *NewBlockEvent) DecodeRLP(s *rlp.Stream) error {
	var block struct {
		Block *types.Block
	}

	if err := s.Decode(&block); err != nil {
		return err
	}
	e.Block = block.Block
	return nil
}

// @todo this doesn't need address. Rework this later to removed address
type RelayBlockEvent struct {
	Hash    common.Hash
	Address common.Address
}

// EncodeRLP serializes b into the Ethereum RLP format.
func (e *RelayBlockEvent) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{e.Hash, e.Address})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (e *RelayBlockEvent) DecodeRLP(s *rlp.Stream) error {
	var header struct {
		Hash    common.Hash
		Address common.Address
	}

	if err := s.Decode(&header); err != nil {
		return err
	}
	e.Hash = header.Hash
	e.Address = header.Address
	return nil
}

type BlameEvent struct {
	Address common.Address
}

// EncodeRLP serializes b into the Ethereum RLP format.
func (e *BlameEvent) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{e.Address})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (e *BlameEvent) DecodeRLP(s *rlp.Stream) error {
	var addr struct {
		Address common.Address
	}

	if err := s.Decode(&addr); err != nil {
		return err
	}
	e.Address = addr.Address
	return nil
}

// @todo this doesn't need address. Rework this later to removed address
type RequestBlockEvent struct {
	Hash    common.Hash
	Address common.Address
}

// EncodeRLP serializes b into the Ethereum RLP format.
func (e *RequestBlockEvent) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{e.Hash, e.Address})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (e *RequestBlockEvent) DecodeRLP(s *rlp.Stream) error {
	var request struct {
		Hash    common.Hash
		Address common.Address
	}

	if err := s.Decode(&request); err != nil {
		return err
	}
	e.Hash = request.Hash
	e.Address = request.Address
	return nil
}

type RespondToRequestEvent struct {
	Block *types.Block
}

// EncodeRLP serializes b into the Ethereum RLP format.
func (e *RespondToRequestEvent) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{e.Block})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (e *RespondToRequestEvent) DecodeRLP(s *rlp.Stream) error {
	var response struct {
		Block *types.Block
	}

	if err := s.Decode(&response); err != nil {
		return err
	}
	e.Block = response.Block
	return nil
}
