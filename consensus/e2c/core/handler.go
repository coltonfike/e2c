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

package core

// Start implements core.Engine.Start
func (c *core) Start() error {
	// Start a new round from last sequence + 1

	// Tests will handle events itself, so we have to make subscribeEvents()
	// be able to call in test.

	return nil
}

// Stop implements core.Engine.Stop
func (c *core) Stop() error {

	// Make sure the handler goroutine exits
	c.handlerWg.Wait()
	return nil
}

// ----------------------------------------------------------------------------
