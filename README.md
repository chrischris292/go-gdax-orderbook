# go-gdax-orderbook
Open sourcing the orderbook code of my auto trader.

## How to run
At project root. Download dependencies
```
  go get -u ./... 
```
Run program
```
  CONFIGOR_ENV=production go run cmd/orderbook/main.go
```

## Dependencies
Has sentry integration...using this is optional.
