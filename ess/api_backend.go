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

package ess

import (
	"context"
	"math/big"

	"github.com/orangeAndSuns/essentia/accounts"
	"github.com/orangeAndSuns/essentia/common"
	"github.com/orangeAndSuns/essentia/common/math"
	"github.com/orangeAndSuns/essentia/core"
	"github.com/orangeAndSuns/essentia/core/bloombits"
	"github.com/orangeAndSuns/essentia/core/rawdb"
	"github.com/orangeAndSuns/essentia/core/state"
	"github.com/orangeAndSuns/essentia/core/types"
	"github.com/orangeAndSuns/essentia/core/vm"
	"github.com/orangeAndSuns/essentia/ess/downloader"
	"github.com/orangeAndSuns/essentia/ess/gasprice"
	"github.com/orangeAndSuns/essentia/essdb"
	"github.com/orangeAndSuns/essentia/event"
	"github.com/orangeAndSuns/essentia/params"
	"github.com/orangeAndSuns/essentia/rpc"
)

// EssAPIBackend implements essapi.Backend for full nodes
type EssAPIBackend struct {
	ess *Essentia
	gpo *gasprice.Oracle
}

// ChainConfig returns the active chain configuration.
func (b *EssAPIBackend) ChainConfig() *params.ChainConfig {
	return b.ess.chainConfig
}

func (b *EssAPIBackend) CurrentBlock() *types.Block {
	return b.ess.blockchain.CurrentBlock()
}

func (b *EssAPIBackend) SetHead(number uint64) {
	b.ess.protocolManager.downloader.Cancel()
	b.ess.blockchain.SetHead(number)
}

func (b *EssAPIBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.ess.miner.PendingBlock()
		return block.Header(), nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.ess.blockchain.CurrentBlock().Header(), nil
	}
	return b.ess.blockchain.GetHeaderByNumber(uint64(blockNr)), nil
}

func (b *EssAPIBackend) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	return b.ess.blockchain.GetHeaderByHash(hash), nil
}

func (b *EssAPIBackend) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.ess.miner.PendingBlock()
		return block, nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.ess.blockchain.CurrentBlock(), nil
	}
	return b.ess.blockchain.GetBlockByNumber(uint64(blockNr)), nil
}

func (b *EssAPIBackend) StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	// Pending state is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block, state := b.ess.miner.Pending()
		return state, block.Header(), nil
	}
	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, blockNr)
	if header == nil || err != nil {
		return nil, nil, err
	}
	stateDb, err := b.ess.BlockChain().StateAt(header.Root)
	return stateDb, header, err
}

func (b *EssAPIBackend) GetBlock(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return b.ess.blockchain.GetBlockByHash(hash), nil
}

func (b *EssAPIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	if number := rawdb.ReadHeaderNumber(b.ess.chainDb, hash); number != nil {
		return rawdb.ReadReceipts(b.ess.chainDb, hash, *number), nil
	}
	return nil, nil
}

func (b *EssAPIBackend) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
	number := rawdb.ReadHeaderNumber(b.ess.chainDb, hash)
	if number == nil {
		return nil, nil
	}
	receipts := rawdb.ReadReceipts(b.ess.chainDb, hash, *number)
	if receipts == nil {
		return nil, nil
	}
	logs := make([][]*types.Log, len(receipts))
	for i, receipt := range receipts {
		logs[i] = receipt.Logs
	}
	return logs, nil
}

func (b *EssAPIBackend) GetTd(blockHash common.Hash) *big.Int {
	return b.ess.blockchain.GetTdByHash(blockHash)
}

func (b *EssAPIBackend) GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types.Header, vmCfg vm.Config) (*vm.EVM, func() error, error) {
	state.SetBalance(msg.From(), math.MaxBig256)
	vmError := func() error { return nil }

	context := core.NewEVMContext(msg, header, b.ess.BlockChain(), nil)
	return vm.NewEVM(context, state, b.ess.chainConfig, vmCfg), vmError, nil
}

func (b *EssAPIBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.ess.BlockChain().SubscribeRemovedLogsEvent(ch)
}

func (b *EssAPIBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.ess.BlockChain().SubscribeChainEvent(ch)
}

func (b *EssAPIBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.ess.BlockChain().SubscribeChainHeadEvent(ch)
}

func (b *EssAPIBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.ess.BlockChain().SubscribeChainSideEvent(ch)
}

func (b *EssAPIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.ess.BlockChain().SubscribeLogsEvent(ch)
}

func (b *EssAPIBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.ess.txPool.AddLocal(signedTx)
}

func (b *EssAPIBackend) GetPoolTransactions() (types.Transactions, error) {
	pending, err := b.ess.txPool.Pending()
	if err != nil {
		return nil, err
	}
	var txs types.Transactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	return txs, nil
}

func (b *EssAPIBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	return b.ess.txPool.Get(hash)
}

func (b *EssAPIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.ess.txPool.State().GetNonce(addr), nil
}

func (b *EssAPIBackend) Stats() (pending int, queued int) {
	return b.ess.txPool.Stats()
}

func (b *EssAPIBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return b.ess.TxPool().Content()
}

func (b *EssAPIBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.ess.TxPool().SubscribeNewTxsEvent(ch)
}

func (b *EssAPIBackend) Downloader() *downloader.Downloader {
	return b.ess.Downloader()
}

func (b *EssAPIBackend) ProtocolVersion() int {
	return b.ess.EssVersion()
}

func (b *EssAPIBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return b.gpo.SuggestPrice(ctx)
}

func (b *EssAPIBackend) ChainDb() essdb.Database {
	return b.ess.ChainDb()
}

func (b *EssAPIBackend) EventMux() *event.TypeMux {
	return b.ess.EventMux()
}

func (b *EssAPIBackend) AccountManager() *accounts.Manager {
	return b.ess.AccountManager()
}

func (b *EssAPIBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := b.ess.bloomIndexer.Sections()
	return params.BloomBitsBlocks, sections
}

func (b *EssAPIBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for i := 0; i < bloomFilterThreads; i++ {
		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, b.ess.bloomRequests)
	}
}
