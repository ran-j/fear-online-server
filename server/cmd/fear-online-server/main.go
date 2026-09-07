package main

import (
	"go-service-template/internal/catalog"
	"go-service-template/internal/configuration"
	"go-service-template/internal/database"
	healthCheckRouter "go-service-template/internal/healthcheck/routes"
	launcherRouter "go-service-template/internal/launcher/routes"
	"go-service-template/internal/middlewares"
	"go-service-template/internal/proudnet"
	"go-service-template/internal/repositories"
	"go-service-template/internal/services"
	"go-service-template/pkg/clock"
	Logger "go-service-template/pkg/logger"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	config, err := configuration.Load()
	if err != nil {
		log.Fatal(err)
	}
	isDevelopment := config.IsDevelopmentEnvironment()

	logger := Logger.New(
		config.ServiceName,
		clock.Clock{},
		isDevelopment,
	)

	db, err := database.Connect(config.MongoURI, config.MongoDatabase)
	if err != nil {
		log.Fatalf("mongodb unreachable at %s: %v", config.MongoURI, err)
	}

	gameCatalog, err := catalog.Load()
	if err != nil {
		log.Fatalf("failed to load game catalog: %v", err)
	}
	logger.Info("catalog loaded: items, maps, rewards, classes, recipes")

	app := fiber.New(fiber.Config{
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	})

	app.Use(recover.New(recover.Config{EnableStackTrace: isDevelopment}))
	app.Use(middlewares.RequestLogger(logger))

	playerRepo := repositories.NewMongoPlayerRepository(db.Collection("players"))
	playerService := services.NewPlayerService(
		playerRepo,
		gameCatalog,
		config.StartingPoint,
		config.StartingCash,
	)
	friendService := services.NewFriendService(playerRepo)
	clanService := services.NewClanService(repositories.NewMongoClanRepository(db.Collection("clans")), playerRepo)

	healthCheckRouter.RegisterRoutes(app, logger, config)
	launcherRouter.RegisterRoutes(app, logger, config, playerService)

	keyPair, err := proudnet.GenerateKeyPair()
	if err != nil {
		log.Fatalf("failed to generate proudnet key pair: %v", err)
	}
	loginPort, err := strconv.Atoi(config.LoginServerPort)
	if err != nil {
		log.Fatalf("invalid LOGIN_SERVER_PORT %q: %v", config.LoginServerPort, err)
	}
	gameServer := proudnet.NewServer(logger, keyPair, config.LoginServerIP, uint16(loginPort))
	channels := []proudnet.Channel{{
		ServerID: 1001,
		ID:       1,
		Kind:     1,
		Name:     "Z_General", // Loki.strdb id, not display text
		Host:     config.LoginServerIP,
		Port:     uint16(loginPort),
		MaxUsers: 100,
	}}
	credentials := proudnet.NewCredentials()
	proudnet.NewLoginHandlers(playerService, clanService, logger, channels, credentials).Register(gameServer)
	roomRegistry := proudnet.NewRoomRegistry()
	gameServer.UseRooms(roomRegistry)
	proudnet.NewLobby(logger, gameCatalog, playerService, roomRegistry).Register(gameServer)
	proudnet.NewItems(playerService, logger).Register(gameServer)
	proudnet.NewClan(clanService, playerService, logger).Register(gameServer)
	proudnet.NewFriends(friendService, clanService, logger).Register(gameServer)
	proudnet.NewChat(playerService, clanService, logger).Register(gameServer)
	proudnet.NewGameActionHandle(gameServer)


	if err := gameServer.Listen(":" + config.LoginServerPort); err != nil {
		log.Fatalf("proudnet listen on :%s failed: %v", config.LoginServerPort, err)
	}
	logger.Info("ProudNet game-login server listening on port " + config.LoginServerPort)

	go func() {
		logger.Info("FEAR Online Server listening on port " + config.Port)
		if err := app.Listen(":" + config.Port); err != nil {
			logger.Error("server stopped: " + err.Error())
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down...")
	if err := app.Shutdown(); err != nil {
		logger.Error("graceful shutdown failed: " + err.Error())
	}
	if err := gameServer.Close(); err != nil {
		logger.Error("proudnet close failed: " + err.Error())
	}
	if err := db.Disconnect(); err != nil {
		logger.Error("mongodb disconnect failed: " + err.Error())
	}
}
