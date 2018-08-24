// Copyright 2015 The qwerty123 Authors
// This file is part of the qwerty123 library.
//
// The qwerty123 library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The qwerty123 library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the qwerty123 library. If not, see <http://www.gnu.org/licenses/>.

// Contains the metrics collected by the downloader.

package downloader

import (
	"github.com/orangeAndSuns/essentia/metrics"
)

var (
	headerInMeter      = metrics.NewRegisteredMeter("ess/downloader/headers/in", nil)
	headerReqTimer     = metrics.NewRegisteredTimer("ess/downloader/headers/req", nil)
	headerDropMeter    = metrics.NewRegisteredMeter("ess/downloader/headers/drop", nil)
	headerTimeoutMeter = metrics.NewRegisteredMeter("ess/downloader/headers/timeout", nil)

	bodyInMeter      = metrics.NewRegisteredMeter("ess/downloader/bodies/in", nil)
	bodyReqTimer     = metrics.NewRegisteredTimer("ess/downloader/bodies/req", nil)
	bodyDropMeter    = metrics.NewRegisteredMeter("ess/downloader/bodies/drop", nil)
	bodyTimeoutMeter = metrics.NewRegisteredMeter("ess/downloader/bodies/timeout", nil)

	receiptInMeter      = metrics.NewRegisteredMeter("ess/downloader/receipts/in", nil)
	receiptReqTimer     = metrics.NewRegisteredTimer("ess/downloader/receipts/req", nil)
	receiptDropMeter    = metrics.NewRegisteredMeter("ess/downloader/receipts/drop", nil)
	receiptTimeoutMeter = metrics.NewRegisteredMeter("ess/downloader/receipts/timeout", nil)

	stateInMeter   = metrics.NewRegisteredMeter("ess/downloader/states/in", nil)
	stateDropMeter = metrics.NewRegisteredMeter("ess/downloader/states/drop", nil)
)
