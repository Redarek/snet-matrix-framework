package app

import (
	"github.com/rs/zerolog/log"
	"github.com/tensved/snet-matrix-framework/internal/config"
	"github.com/tensved/snet-matrix-framework/internal/grpc_manager"
	"github.com/tensved/snet-matrix-framework/internal/logger"
	"github.com/tensved/snet-matrix-framework/internal/matrix"
	"github.com/tensved/snet-matrix-framework/internal/server"
	"github.com/tensved/snet-matrix-framework/internal/snet_syncer"
	"github.com/tensved/snet-matrix-framework/pkg/blockchain"
	"github.com/tensved/snet-matrix-framework/pkg/db"
	ipfs "github.com/tensved/snet-matrix-framework/pkg/ipfs"
	"maunium.net/go/mautrix/event"
	"time"
)

type App struct {
	DB           db.Service
	Fiber        *server.FiberServer
	Ethereum     blockchain.Ethereum
	MatrixClient matrix.Service
	IPFSClient   ipfs.IPFSClient
	Syncer       snet_syncer.SnetSyncer
	GRPCManager  *grpc_manager.GRPCClientManager
}

func New() App {
	logger.Setup()
	config.Init()
	database := db.New()
	eth := blockchain.Init()
	ipfsClient := ipfs.Init()
	snetSyncer := snet_syncer.New(eth, ipfsClient, database)
	grpcManager := grpc_manager.NewGRPCClientManager()
	app := App{DB: database, Fiber: server.New(database), MatrixClient: matrix.New(database, snetSyncer, grpcManager, eth), IPFSClient: ipfsClient, Ethereum: eth, Syncer: snetSyncer, GRPCManager: grpcManager}

	app.Syncer.DB = app.DB
	app.Syncer.Ethereum = app.Ethereum
	app.Syncer.IPFSClient = app.IPFSClient

	return app
}

func (app App) Run() {
	var err error
	app.MatrixClient.Auth()
	ch := make(chan *event.Event)
	err = app.MatrixClient.StartListening(ch)
	if err != nil {
		log.Error().Err(err).Msg("Failed to start matrix event listener")
		return
	}

	go app.Syncer.Start()

	go func() {
		ticker := time.NewTicker(3 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// Call the Auth function every time the ticker ticks.
				app.MatrixClient.Auth()
			}
		}
	}()

	app.Fiber.RegisterFiberRoutes()
	err = app.Fiber.App.Listen(":" + config.App.Port)
	if err != nil {
		log.Error().Err(err).Msg("Failed to start fiber server")
	}
}
