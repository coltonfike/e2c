package main

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	atypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

func Encode(vanity string, validators common.Address) (string, error) {
	newVanity, err := hexutil.Decode(vanity)
	if err != nil {
		return "", err
	}

	if len(newVanity) < atypes.E2CExtraVanity {
		newVanity = append(newVanity, bytes.Repeat([]byte{0x00}, atypes.E2CExtraVanity-len(newVanity))...)
	}
	newVanity = newVanity[:atypes.E2CExtraVanity]

	ist := &atypes.E2CExtra{
		Leader:        validators,
		Seal:          make([]byte, atypes.E2CExtraSeal),
		CommittedSeal: [][]byte{},
	}

	payload, err := rlp.EncodeToBytes(&ist)
	if err != nil {
		return "", err
	}

	return "0x" + common.Bytes2Hex(append(newVanity, payload...)), nil
}

func main() {
	addr := common.HexToAddress("0x8536571507DD25f52E0955e80bcD59c18427A371")
	fmt.Println(Encode("0x00", addr))
}
