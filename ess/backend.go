// Copyright 2014 The qwerty123 Authors
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

// Package ess implements the Essentia protocol.
package ess

import (
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/orangeAndSuns/essentia/accounts"
	"github.com/orangeAndSuns/essentia/common"
	"github.com/orangeAndSuns/essentia/common/hexutil"
	"github.com/orangeAndSuns/essentia/consensus"
	"github.com/orangeAndSuns/essentia/consensus/clique"
	"github.com/orangeAndSuns/essentia/consensus/esshash"
	"github.com/orangeAndSuns/essentia/core"
	"github.com/orangeAndSuns/essentia/core/bloombits"
	"github.com/orangeAndSuns/essentia/core/rawdb"
	"github.com/orangeAndSuns/essentia/core/types"
	"github.com/orangeAndSuns/essentia/core/vm"
	"github.com/orangeAndSuns/essentia/ess/downloader"
	"github.com/orangeAndSuns/essentia/ess/filters"
	"github.com/orangeAndSuns/essentia/ess/gasprice"
	"github.com/orangeAndSuns/essentia/essdb"
	"github.com/orangeAndSuns/essentia/event"
	"github.com/orangeAndSuns/essentia/internal/essapi"
	"github.com/orangeAndSuns/essentia/log"
	"github.com/orangeAndSuns/essentia/miner"
	"github.com/orangeAndSuns/essentia/node"
	"github.com/orangeAndSuns/essentia/p2p"
	"github.com/orangeAndSuns/essentia/params"
	"github.com/orangeAndSuns/essentia/rlp"
	"github.com/orangeAndSuns/essentia/rpc"
)

type LesServer interface {
	Start(srvr *p2p.Server)
	Stop()
	Protocols() []p2p.Protocol
	SetBloomBitsIndexer(bbIndexer *core.ChainIndexer)
}

// Essentia implements the Essentia full node service.
type Essentia struct {
	config      *Config
	chainConfig *params.ChainConfig

	// Channel for shutting down the service
	shutdownChan chan bool // Channel for shutting down the Essentia

	// Handlers
	txPool          *core.TxPool
	blockchain      *core.BlockChain
	protocolManager *ProtocolManager
	lesServer       LesServer

	// DB interfaces
	chainDb essdb.Database // Block chain database

	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager

	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer  *core.ChainIndexer             // Bloom indexer operating during block imports

	APIBackend *EssAPIBackend

	miner    *miner.Miner
	gasPrice *big.Int
	essbase  common.Address

	networkID     uint64
	netRPCService *essapi.PublicNetAPI

	lock sync.RWMutex // Protects the variadic fields (e.g. gas price and essbase)
}

func (s *Essentia) AddLesServer(ls LesServer) {
	s.lesServer = ls
	ls.SetBloomBitsIndexer(s.bloomIndexer)
}

// New creates a new Essentia object (including the
// initialisation of the common Essentia object)
func New(ctx *node.ServiceContext, config *Config) (*Essentia, error) {
	if config.SyncMode == downloader.LightSync {
		return nil, errors.New("can't run ess.Essentia in light sync mode, use les.LightEssentia")
	}
	if !config.SyncMode.IsValid() {
		return nil, fmt.Errorf("invalid sync mode %d", config.SyncMode)
	}
	chainDb, err := CreateDB(ctx, config, "chaindata")
	if err != nil {
		return nil, err
	}
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlock(chainDb, config.Genesis)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)
	log.Info("ESShash", "difficulty", config.ESShash.Difficulty)

	ess := &Essentia{
		config:         config,
		chainDb:        chainDb,
		chainConfig:    chainConfig,
		eventMux:       ctx.EventMux,
		accountManager: ctx.AccountManager,
		engine:         CreateConsensusEngine(ctx, &config.ESShash, chainConfig, chainDb),
		shutdownChan:   make(chan bool),
		networkID:      config.NetworkId,
		gasPrice:       config.GasPrice,
		essbase:        config.ESSBase,
		bloomRequests:  make(chan chan *bloombits.Retrieval),
		bloomIndexer:   NewBloomIndexer(chainDb, params.BloomBitsBlocks),
	}

	log.Info("Initialising Essentia protocol", "versions", ProtocolVersions, "network", config.NetworkId)

	if !config.SkipBcVersionCheck {
		bcVersion := rawdb.ReadDatabaseVersion(chainDb)
		if bcVersion != core.BlockChainVersion && bcVersion != 0 {
			return nil, fmt.Errorf("Blockchain DB version mismatch (%d / %d). Run gess upgradedb.\n", bcVersion, core.BlockChainVersion)
		}
		rawdb.WriteDatabaseVersion(chainDb, core.BlockChainVersion)
	}
	var (
		vmConfig    = vm.Config{EnablePreimageRecording: config.EnablePreimageRecording}
		cacheConfig = &core.CacheConfig{Disabled: config.NoPruning, TrieNodeLimit: config.TrieCache, TrieTimeLimit: config.TrieTimeout}
	)
	ess.blockchain, err = core.NewBlockChain(chainDb, cacheConfig, ess.chainConfig, ess.engine, vmConfig)
	if err != nil {
		return nil, err
	}
	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		ess.blockchain.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}
	ess.bloomIndexer.Start(ess.blockchain)

	if config.TxPool.Journal != "" {
		config.TxPool.Journal = ctx.ResolvePath(config.TxPool.Journal)
	}
	ess.txPool = core.NewTxPool(config.TxPool, ess.chainConfig, ess.blockchain)

	if ess.protocolManager, err = NewProtocolManager(ess.chainConfig, config.SyncMode, config.NetworkId, ess.eventMux, ess.txPool, ess.engine, ess.blockchain, chainDb); err != nil {
		return nil, err
	}
	ess.miner = miner.New(ess, ess.chainConfig, ess.EventMux(), ess.engine)
	ess.miner.SetExtra(makeExtraData(config.ExtraData))

	ess.APIBackend = &EssAPIBackend{ess, nil}
	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.GasPrice
	}
	ess.APIBackend.gpo = gasprice.NewOracle(ess.APIBackend, gpoParams)

	return ess, nil
}

func makeExtraData(extra []byte) []byte {
	if len(extra) == 0 {
		// create default extradata
		extra, _ = rlp.EncodeToBytes([]interface{}{
			uint(params.VersionMajor<<16 | params.VersionMinor<<8 | params.VersionPatch),
			"gess",
			runtime.Version(),
			runtime.GOOS,
		})
	}
	if uint64(len(extra)) > params.MaximumExtraDataSize {
		log.Warn("Miner extra data exceed limit", "extra", hexutil.Bytes(extra), "limit", params.MaximumExtraDataSize)
		extra = nil
	}
	return extra
}

// CreateDB creates the chain database.
func CreateDB(ctx *node.ServiceContext, config *Config, name string) (essdb.Database, error) {
	db, err := ctx.OpenDatabase(name, config.DatabaseCache, config.DatabaseHandles)
	if err != nil {
		return nil, err
	}
	if db, ok := db.(*essdb.LDBDatabase); ok {
		db.Meter("ess/db/chaindata/")
	}
	return db, nil
}

// CreateConsensusEngine creates the required type of consensus engine instance for an Essentia service
func CreateConsensusEngine(ctx *node.ServiceContext, config *esshash.Config, chainConfig *params.ChainConfig, db essdb.Database) consensus.Engine {
	// If proof-of-authority is requested, set it up
	if chainConfig.Clique != nil {
		return clique.New(chainConfig.Clique, db)
	}
	// Otherwise assume proof-of-work
	switch config.PowMode {
	case esshash.ModeFake:
		log.Warn("ESShash used in fake mode")
		return esshash.NewFaker()
	case esshash.ModeTest:
		log.Warn("ESShash used in test mode")
		return esshash.NewTester()
	case esshash.ModeShared:
		log.Warn("ESShash used in shared mode")
		return esshash.NewShared()
	default:
		engine := esshash.New(esshash.Config{
			CacheDir:       ctx.ResolvePath(config.CacheDir),
			CachesInMem:    config.CachesInMem,
			CachesOnDisk:   config.CachesOnDisk,
			DatasetDir:     config.DatasetDir,
			DatasetsInMem:  config.DatasetsInMem,
			DatasetsOnDisk: config.DatasetsOnDisk,
		})
		engine.SetThreads(-1) // Disable CPU mining
		return engine
	}
}

// APIs return the collection of RPC services the essentia package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *Essentia) APIs() []rpc.API {
	apis := essapi.GetAPIs(s.APIBackend)

	// Append any APIs exposed explicitly by the consensus engine
	apis = append(apis, s.engine.APIs(s.BlockChain())...)

	// Append all the local APIs and return
	return append(apis, []rpc.API{
		{
			Namespace: "ess",
			Version:   "1.0",
			Service:   NewPublicEssentiaAPI(s),
			Public:    true,
		}, {
			Namespace: "ess",
			Version:   "1.0",
			Service:   NewPublicMinerAPI(s),
			Public:    true,
		}, {
			Namespace: "ess",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.protocolManager.downloader, s.eventMux),
			Public:    true,
		}, {
			Namespace: "miner",
			Version:   "1.0",
			Service:   NewPrivateMinerAPI(s),
			Public:    false,
		}, {
			Namespace: "ess",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.APIBackend, false),
			Public:    true,
		}, {
			Namespace: "admin",
			Version:   "1.0",
			Service:   NewPrivateAdminAPI(s),
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPublicDebugAPI(s),
			Public:    true,
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPrivateDebugAPI(s.chainConfig, s),
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		},
	}...)
}

func (s *Essentia) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *Essentia) ESSBase() (eb common.Address, err error) {
	s.lock.RLock()
	essbase := s.essbase
	s.lock.RUnlock()

	if essbase != (common.Address{}) {
		return essbase, nil
	}
	if wallets := s.AccountManager().Wallets(); len(wallets) > 0 {
		if accounts := wallets[0].Accounts(); len(accounts) > 0 {
			essbase := accounts[0].Address

			s.lock.Lock()
			s.essbase = essbase
			s.lock.Unlock()

			log.Info("ESSBase automatically configured", "address", essbase)
			return essbase, nil
		}
	}
	return common.Address{}, fmt.Errorf("essbase must be explicitly specified")
}

// SetESSBase sets the mining reward address.
func (s *Essentia) SetESSBase(essbase common.Address) {
	s.lock.Lock()
	s.essbase = essbase
	s.lock.Unlock()

	s.miner.SetESSBase(essbase)
}

func (s *Essentia) StartMining(local bool) error {
	eb, err := s.ESSBase()
	if err != nil {
		log.Error("Cannot start mining without essbase", "err", err)
		return fmt.Errorf("essbase missing: %v", err)
	}
	if clique, ok := s.engine.(*clique.Clique); ok {
		wallet, err := s.accountManager.Find(accounts.Account{Address: eb})
		if wallet == nil || err != nil {
			log.Error("ESSBase account unavailable locally", "err", err)
			return fmt.Errorf("signer missing: %v", err)
		}
		clique.Authorize(eb, wallet.SignHash)
	}
	if local {
		// If local (CPU) mining is started, we can disable the transaction rejection
		// mechanism introduced to speed sync times. CPU mining on mainnet is ludicrous
		// so none will ever hit this path, whereas marking sync done on CPU mining
		// will ensure that private networks work in single miner mode too.
		atomic.StoreUint32(&s.protocolManager.acceptTxs, 1)
	}
	go s.miner.Start(eb)
	return nil
}

func (s *Essentia) StopMining()         { s.miner.Stop() }
func (s *Essentia) IsMining() bool      { return s.miner.Mining() }
func (s *Essentia) Miner() *miner.Miner { return s.miner }

func (s *Essentia) AccountManager() *accounts.Manager  { return s.accountManager }
func (s *Essentia) BlockChain() *core.BlockChain       { return s.blockchain }
func (s *Essentia) TxPool() *core.TxPool               { return s.txPool }
func (s *Essentia) EventMux() *event.TypeMux           { return s.eventMux }
func (s *Essentia) Engine() consensus.Engine           { return s.engine }
func (s *Essentia) ChainDb() essdb.Database            { return s.chainDb }
func (s *Essentia) IsListening() bool                  { return true } // Always listening
func (s *Essentia) EssVersion() int                    { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *Essentia) NetVersion() uint64                 { return s.networkID }
func (s *Essentia) Downloader() *downloader.Downloader { return s.protocolManager.downloader }

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *Essentia) Protocols() []p2p.Protocol {
	if s.lesServer == nil {
		return s.protocolManager.SubProtocols
	}
	return append(s.protocolManager.SubProtocols, s.lesServer.Protocols()...)
}

// Start implements node.Service, starting all internal goroutines needed by the
// Essentia protocol implementation.
func (s *Essentia) Start(srvr *p2p.Server) error {
	// Start the bloom bits servicing goroutines
	s.startBloomHandlers()

	// Start the RPC service
	s.netRPCService = essapi.NewPublicNetAPI(srvr, s.NetVersion())

	// Figure out a max peers count based on the server limits
	maxPeers := srvr.MaxPeers
	if s.config.LightServ > 0 {
		if s.config.LightPeers >= srvr.MaxPeers {
			return fmt.Errorf("invalid peer config: light peer count (%d) >= total peer count (%d)", s.config.LightPeers, srvr.MaxPeers)
		}
		maxPeers -= s.config.LightPeers
	}
	// Start the networking layer and the light server if requested
	s.protocolManager.Start(maxPeers)
	if s.lesServer != nil {
		s.lesServer.Start(srvr)
	}
	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// Essentia protocol.
func (s *Essentia) Stop() error {
	s.bloomIndexer.Close()
	s.blockchain.Stop()
	s.protocolManager.Stop()
	if s.lesServer != nil {
		s.lesServer.Stop()
	}
	s.txPool.Stop()
	s.miner.Stop()
	s.eventMux.Stop()

	s.chainDb.Close()
	close(s.shutdownChan)

	return nil
}
