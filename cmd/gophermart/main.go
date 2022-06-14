package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"diploma/cmd/handlers"
	"diploma/cmd/middleware"
	"diploma/cmd/settings"
	"diploma/cmd/storage"

	"github.com/caarlos0/env"
	"github.com/gin-gonic/gin"
)

func setupGinHandlers(s *storage.Storage, cs *storage.CookieStorage) *gin.Engine {
	router := gin.New()
	router.Use(
		gin.Recovery(),
		gin.Logger(),
		middleware.Compression(),
		middleware.Decompression(),
		middleware.Authentication(cs),
	)

	router.GET("/ping", handlers.PingDB(s))
	router.GET("/api/user/balance", handlers.GetBalance(s, cs))
	router.GET("/api/user/balance/withdrawals",handlers.GetWithdrawals(s, cs))
	router.GET("/api/user/orders", handlers.GetOrders(s, cs))
	router.POST("/api/user/register", handlers.Register(s, cs))
	router.POST("/api/user/login", handlers.Login(s, cs))
	router.POST("/api/user/balance/withdraw", handlers.PlaceWithdrawOrder(s, cs))
	router.POST("/api/user/orders", handlers.PlaceOrder(s, cs))

	return router
}

func main() {
	// Init config, parse flags, init base context
	wg := new(sync.WaitGroup)
	wg.Add(1)
	config := new(settings.Config)
	err := env.Parse(config)
	if err != nil {
		log.Println("Env parsing ERR", err)
	}
	flag.StringVar(&config.ServerAddress, "a", "localhost:8080", "Server address")
	flag.StringVar(&config.DatabaseURI, "d", "postgres://postgres:example@localhost:5432", "Postgress connection uri")
	flag.StringVar(&config.AccrualAddress, "r", "", "Accrual system address")
	flag.Parse()
	log.Println("\n\n\n\n\n\n\n\n", config, "------------------------------------------")
	// Init storages
	ctx, cancel := context.WithCancel(context.Background())
	dbStore, err := storage.NewStorageConnection(ctx, config.DatabaseURI)
	if err != nil {
		panic(err)
	}
	err = dbStore.PrepareWorkingTables()
	if err != nil {
		panic(err)
	}
	cs := storage.NewCS(6)

	// Setup gin engine, accrual api and start server
	router := setupGinHandlers(dbStore, cs)
	server := &http.Server{
		Addr:    config.ServerAddress,
		Handler: router,
	}
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			log.Println("Listen: ", err)
		}
	}()
	go handlers.StartAccrualAPI(ctx, dbStore, wg)

	// Exit on syscalls
	syscallCancelChan := make(chan os.Signal, 1)
	signal.Notify(syscallCancelChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	sig := <-syscallCancelChan
	log.Println("Caught syscall:", sig)
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
	log.Println("Server exiting")
	cancel()
	wg.Wait()
}
