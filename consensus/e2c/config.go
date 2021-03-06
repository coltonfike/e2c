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

import "time"

type Config struct {
	Delta                  time.Duration `toml:",omitempty"` // Network speed
	BlockSize              uint64        `toml:",omitempty"` // Determines how many transactions go in each block
	AllowedFutureBlockTime uint64        `toml:",omitempty"` // This is required by miner, even though we don't use it
}

var DefaultConfig = &Config{
	Delta:                  200,
	BlockSize:              200,
	AllowedFutureBlockTime: 0,
}
