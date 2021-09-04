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

package types

import (
	"errors"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	// E2CDigest represents a hash of "E2C practical byzantine fault tolerance"
	// to identify whether the block is from E2C consensus engine
	E2CDigest = common.HexToHash(crypto.Keccak256Hash([]byte("E2C practical byzantine fault tolerance")).String())

	E2CExtraVanity = 32                     // Fixed number of extra-data bytes reserved for validator vanity
	E2CExtraSeal   = crypto.SignatureLength // Fixed number of extra-data bytes reserved for validator seal

	// ErrInvalidE2CHeaderExtra is returned if the length of extra-data is less than 32 bytes
	ErrInvalidE2CHeaderExtra = errors.New("invalid e2c header extra-data")
)

type E2CExtra struct {
	Validators []common.Address
	Seal       []byte
}

// EncodeRLP serializes ist into the Ethereum RLP format.
func (ist *E2CExtra) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		ist.Validators,
		ist.Seal,
	})
}

// DecodeRLP implements rlp.Decoder, and load the istanbul fields from a RLP stream.
func (ist *E2CExtra) DecodeRLP(s *rlp.Stream) error {
	var E2CExtra struct {
		Validators []common.Address
		Seal       []byte
	}
	if err := s.Decode(&E2CExtra); err != nil {
		return err
	}
	ist.Validators, ist.Seal = E2CExtra.Validators, E2CExtra.Seal
	return nil
}

// ExtractE2CExtra extracts all values of the E2CExtra from the header. It returns an
// error if the length of the given extra-data is less than 32 bytes or the extra-data can not
// be decoded.
func ExtractE2CExtra(h *Header) (*E2CExtra, error) {
	if len(h.Extra) < E2CExtraVanity {
		return nil, ErrInvalidE2CHeaderExtra
	}

	var E2CExtra *E2CExtra
	err := rlp.DecodeBytes(h.Extra[E2CExtraVanity:], &E2CExtra)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return E2CExtra, nil
}

// E2CFilteredHeader returns a filtered header which some information (like seal, committed seals)
// are clean to fulfill the E2C hash rules. It returns nil if the extra-data cannot be
// decoded/encoded by rlp.
func E2CFilteredHeader(h *Header, keepSeal bool) *Header {
	newHeader := CopyHeader(h)
	E2CExtra, err := ExtractE2CExtra(newHeader)
	if err != nil {
		return nil
	}

	if !keepSeal {
		E2CExtra.Seal = []byte{}
	}

	payload, err := rlp.EncodeToBytes(&E2CExtra)
	if err != nil {
		return nil
	}

	newHeader.Extra = append(newHeader.Extra[:E2CExtraVanity], payload...)

	return newHeader
}
