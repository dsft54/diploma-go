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

	"github.com/caarlos0/env"
	"github.com/gin-gonic/gin"

	"github.com/dsft54/gophermart/internal/pkg/handlers"
	"github.com/dsft54/gophermart/internal/pkg/middleware"
	"github.com/dsft54/gophermart/internal/pkg/settings"
	"github.com/dsft54/gophermart/internal/pkg/storage"
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
	router.GET("/api/user/balance/withdrawals", handlers.GetWithdrawals(s, cs))
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
	defer wg.Wait()
	config := new(settings.Config)
	err := env.Parse(config)
	if err != nil {
		log.Println("Env parsing ERR", err)
	}
	flag.StringVar(&config.ServerAddress, "a", config.ServerAddress, "Server address")
	flag.StringVar(&config.DatabaseURI, "d", config.DatabaseURI, "Postgress connection uri")
	flag.StringVar(&config.AccrualAddress, "r", config.AccrualAddress, "Accrual system address")
	flag.Parse()
	log.Println(config)

	// Init storages
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
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
	defer func() {
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal("Server Shutdown:", err)
		}
	}()
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		err := server.ListenAndServe()
		if err != nil {
			log.Println("Listen: ", err)
		}
	}(wg)
	wg.Add(1)
	go handlers.StartAccrualAPI(ctx, config.AccrualAddress, dbStore, wg)

	// Exit on syscalls
	syscallCancelChan := make(chan os.Signal, 1)
	signal.Notify(syscallCancelChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	sig := <-syscallCancelChan
	log.Println("Caught syscall:", sig)
	log.Println("Server exiting")
}
