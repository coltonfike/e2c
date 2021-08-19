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
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

type Blame struct {
	View uint64
	// Are these necessary?
	//	Block     *types.Block
	//	BlockStar *types.Block
}

func (b *Blame) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{b.View}) //, b.Block, b.BlockStar})
}

func (b *Blame) DecodeRLP(s *rlp.Stream) error {
	var blame struct {
		View uint64
		// Block     *types.Block
		// BlockStar *types.Block
	}

	if err := s.Decode(&blame); err != nil {
		return err
	}
	// b.View, b.Block, b.BlockStar = blame.View, blame.Block, blame.BlockStar
	b.View = blame.View
	return nil
}

type Vote struct {
	Blocks []*Message
}

func (v *Vote) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{v.Blocks})
}

func (v *Vote) DecodeRLP(s *rlp.Stream) error {
	var vote struct {
		Blocks []*Message
	}

	if err := s.Decode(&vote); err != nil {
		return err
	}
	v.Blocks = vote.Blocks
	return nil
}

type BlameCertificateEvent struct {
	Lock      *types.Block
	Committed *types.Block
	Address   common.Address
}

type BlockCertificate struct {
	Block *types.Block
	Votes []*Message
}

func (bc *BlockCertificate) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{bc.Block, bc.Votes})
}

func (bc *BlockCertificate) DecodeRLP(s *rlp.Stream) error {
	var cert struct {
		Block *types.Block
		Votes []*Message
	}

	if err := s.Decode(&cert); err != nil {
		return err
	}
	bc.Block, bc.Votes = cert.Block, cert.Votes
	return nil
}

func (bc *BlockCertificate) String() string {
	return fmt.Sprintf("{Number: %v, Hash: %v, Votes: %v}", bc.Block.Number(), bc.Block.Hash().String(), bc.Votes)
}

type BlameCert struct {
	Blames []*Message
}

func (bc *BlameCert) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{bc.Blames})
}

func (bc *BlameCert) DecodeRLP(s *rlp.Stream) error {
	var cert struct {
		Blames []*Message
	}

	if err := s.Decode(&cert); err != nil {
		return err
	}
	bc.Blames = cert.Blames
	return nil
}

type BlameCertificate struct {
	Lock      *types.Block
	Committed *types.Block
}

func (bc *BlameCertificate) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{bc.Lock, bc.Committed})
}

func (bc *BlameCertificate) DecodeRLP(s *rlp.Stream) error {
	var cert struct {
		Lock      *types.Block
		Committed *types.Block
	}

	if err := s.Decode(&cert); err != nil {
		return err
	}
	bc.Lock, bc.Committed = cert.Lock, cert.Committed
	return nil
}

type B1 struct {
	Cert  *BlockCertificate
	Block *types.Block
}

func (b *B1) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{b.Cert, b.Block})
}

func (b *B1) DecodeRLP(s *rlp.Stream) error {
	var cert struct {
		Cert  *BlockCertificate
		Block *types.Block
	}

	if err := s.Decode(&cert); err != nil {
		return err
	}
	b.Cert, b.Block = cert.Cert, cert.Block
	return nil
}

type B2 struct {
	//@todo change this to array of validate messages
	Validates []*Message
	Block     *types.Block
}

func (b *B2) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{b.Validates, b.Block})
}

func (b *B2) DecodeRLP(s *rlp.Stream) error {
	var cert struct {
		Validates []*Message
		Block     *types.Block
	}

	if err := s.Decode(&cert); err != nil {
		return err
	}
	b.Validates, b.Block = cert.Validates, cert.Block
	return nil
}
