package main

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	atypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

func Encode(vanity string, validators []common.Address) (string, error) {
	newVanity, err := hexutil.Decode(vanity)
	if err != nil {
		return "", err
	}

	if len(newVanity) < atypes.E2CExtraVanity {
		newVanity = append(newVanity, bytes.Repeat([]byte{0x00}, atypes.E2CExtraVanity-len(newVanity))...)
	}
	newVanity = newVanity[:atypes.E2CExtraVanity]

	ist := &atypes.E2CExtra{
		Validators: validators,
		Seal:       make([]byte, atypes.E2CExtraSeal),
	}

	payload, err := rlp.EncodeToBytes(&ist)
	if err != nil {
		return "", err
	}

	return "0x" + common.Bytes2Hex(append(newVanity, payload...)), nil
}

func main() {
	addr := []common.Address{
		common.HexToAddress("0396ea1512b97c9f7e90f641a46f48967db064ba"),
		common.HexToAddress("17145655faf1fcbbd0473502c504f32ca9ebe144"),
		common.HexToAddress("1a80a2887c640c886606e34d9cfc48637a5b4ceb"),
		common.HexToAddress("200a7b1d5f2de512c05b0c46eb50e4d2f2922ada")}

	s, _ := Encode("0x00", addr)
	fmt.Println("Extra Data: " + s)
	E2CDigest := common.HexToHash(crypto.Keccak256Hash([]byte("E2C practical byzantine fault tolerance")).String())
	fmt.Println("Mix Hash: " + E2CDigest.String())
}
