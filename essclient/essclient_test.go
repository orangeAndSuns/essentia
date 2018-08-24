// Copyright 2016 The go-ethereum Authors
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

package essclient

import "github.com/orangeAndSuns/essentia"

// Verify that Client implements the essentia interfaces.
var (
	_ = essentia.ChainReader(&Client{})
	_ = essentia.TransactionReader(&Client{})
	_ = essentia.ChainStateReader(&Client{})
	_ = essentia.ChainSyncReader(&Client{})
	_ = essentia.ContractCaller(&Client{})
	_ = essentia.GasEstimator(&Client{})
	_ = essentia.GasPricer(&Client{})
	_ = essentia.LogFilterer(&Client{})
	_ = essentia.PendingStateReader(&Client{})
	// _ = essentia.PendingStateEventer(&Client{})
	_ = essentia.PendingContractCaller(&Client{})
)
