package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/chrischris292/go-gdax-orderbook/common/util"
	"github.com/chrischris292/go-gdax-orderbook/config/orderbook"
	"github.com/chrischris292/go-gdax-orderbook/gdax"
	raven "github.com/getsentry/raven-go"
	"github.com/jinzhu/configor"
	gdaxClient "github.com/preichenberger/go-gdax"
	"go.uber.org/zap"
)

func main() {

	// Setup config/sentry/logging
	err := configor.Load(&config.AppConfig, "config/orderbook/config.yml")
	if err != nil {
		fmt.Printf("Could not load config: %v", err)
		raven.CaptureError(err, nil)
		panic(nil)
	}

	err = raven.SetDSN(config.AppConfig.Sentry.Dsn)
	if err != nil {
		panic(err)
	}
	raven.SetEnvironment(configor.ENV())
	raven.CaptureMessageAndWait("Starting", nil)
	util.InitializeZap(config.AppConfig.ZapConfig)

	ravenErr, _ := raven.CapturePanicAndWait(func() {
		client := gdaxClient.NewClient(config.AppConfig.Coinbase.CB_Secret, config.AppConfig.Coinbase.CB_Key, config.AppConfig.Coinbase.CB_Passphrase)
		client.BaseURL = config.AppConfig.Coinbase.CB_REST_API
		// Initialize Order Book
		book := gdax.NewBook("BTC-USD")
		gdaxHandler := gdax.NewHandler(client, book)
		go gdaxHandler.Run()

		// Block main thread until Sigterm signal received
		c := make(chan os.Signal)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		raven.CaptureMessageAndWait("Shutting down", nil)
	}, nil)
	zap.L().Fatal(fmt.Sprintf("%v", ravenErr))
}
