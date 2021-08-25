package e2c

import (
	"math"

	"github.com/ethereum/go-ethereum/common"
)

type Validators []common.Address

// retrieves the validator index given it's address
func (v Validators) GetByAddress(addr common.Address) (int, common.Address) {
	for i, val := range v {
		if addr == val {
			return i, val
		}
	}
	return -1, common.Address{}
}

func (v Validators) F() uint64 {
	return uint64(math.Ceil(float64(len(v) / 2))) // @todo is this formula correct?
}
