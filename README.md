
# alpaca-trade-api-go

[![CircleCI Status](https://circleci.com/gh/alpacahq/alpaca-trade-api-go.svg?style=svg)](https://circleci.com/gh/alpacahq/alpaca-trade-api-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/market-development-strategy/alpaca-trade-api-go)](https://goreportcard.com/report/github.com/market-development-strategy/alpaca-trade-api-go)

`alpaca-trade-api-go` is a Go library for the Alpaca trade API. It allows rapid 
trading algo development easily, with support for the both REST and streaming interfaces.
 For details of each API behavior, please see the online API document.

## Installation

```
$ go get github.com/market-development-strategy/alpaca-trade-api-go/common
$ go get github.com/market-development-strategy/alpaca-trade-api-go/polygon
$ go get github.com/market-development-strategy/alpaca-trade-api-go/stream
$ go get github.com/market-development-strategy/alpaca-trade-api-go/alpaca
```

## Example

In order to call Alpaca's trade API, you need to obtain an API key pair.
Replace <key_id> and <secret_key> with what you get from the web console.

### REST example

```go
import (
    "os"
    "fmt"

    "github.com/market-development-strategy/alpaca-trade-api-go/alpaca"
    "github.com/market-development-strategy/alpaca-trade-api-go/common"
)

func init() {
    os.Setenv(common.EnvApiKeyID, "<key_id>")
    os.Setenv(common.EnvApiSecretKey, "<secret_key>")

    fmt.Printf("Running w/ credentials [%v %v]\n", common.Credentials().ID, common.Credentials().Secret)

    alpaca.SetBaseUrl("https://paper-api.alpaca.markets")
}

func main() {
    alpacaClient := alpaca.NewClient(common.Credentials())
    acct, err := alpacaClient.GetAccount()
    if err != nil {
        panic(err)
    }

    fmt.Println(*acct)
}
```

### Streaming example

The SDK provides a unified streaming interface for both data updates 
(from Alpaca or Polygon), and Alpaca's trade/account updates. 
The following example subscribes to trade updates, and prints any messages received,
and subscribes to live quotes for AAPL, and prints any quotes received. 
The main function also ends with an empty `select{}` statement which causes the
 program to run indefinitely.

In order to use Polygon streaming, you need to call `stream.SetDataStream("polygon")`.
 This requires your Alpaca account to be eligible for Polygon integration 
 (for details of the setup, please read Alpaca API document).
```go
package main

import (
	"fmt"
	"os"

	"github.com/market-development-strategy/alpaca-trade-api-go/alpaca"
	"github.com/market-development-strategy/alpaca-trade-api-go/common"
	"github.com/market-development-strategy/alpaca-trade-api-go/stream"
)

func main() {
	os.Setenv(common.EnvApiKeyID, "your_key_id")
	os.Setenv(common.EnvApiSecretKey, "your_secret_key")

	if err := stream.Register(alpaca.TradeUpdates, tradeHandler); err != nil {
		panic(err)
	}

	if err := stream.Register("Q.AAPL", quoteHandler); err != nil {
		panic(err)
	}

	select {}
}

func tradeHandler(msg interface{}) {
	tradeupdate := msg.(alpaca.TradeUpdate)
	fmt.Printf("%s event received for order %s.\n", tradeupdate.Event, tradeupdate.Order.ID)
}

func quoteHandler(msg interface{}) {
	quote := msg.(alpaca.StreamQuote)

	fmt.Println(quote.Symbol, quote.BidPrice, quote.BidSize, quote.AskPrice, quote.AskSize)
}
```

#### Deregister
You could also deregister from a channel. e.g:

```go
   if err := stream.Deregister("Q.AAPL"); err != nil {
        panic(err)
      }
```

## API Document

The HTTP API document is located at https://docs.alpaca.markets/

## Authentication

The Alpaca API requires API key ID and secret key, which you can obtain from 
the web console after you sign in. This key pair can then be applied to the SDK
either by setting environment variables (`APCA_API_KEY_ID=<key_id>` and `APCA_API_SECRET_KEY=<secret_key>`), 
or hardcoding them into the Go code directly as shown in the examples above.

```sh
$ export APCA_API_KEY_ID=xxxxx
$ export APCA_API_SECRET_KEY=yyyyy
```

## Endpoint

For paper trading, set the environment variable `APCA_API_BASE_URL`.

```sh
$ export APCA_API_BASE_URL=https://paper-api.alpaca.markets
```

You can also instead use the function `alpaca.SetBaseUrl("https://paper-api.alpaca.markets")` 
to configure the endpoint.


## Running Multiple Strategies
There's a way to execute more than one algorithm at once.<br>
The websocket connection is limited to 1 connection per account. <br>
For that exact purpose this ![project](https://github.com/shlomikushchi/alpaca-proxy-agent) was created<br>
The steps to execute this are:
* Run the Alpaca Proxy Agent as described in the project's README
* Define this env variable: `DATA_PROXY_WS` to be the address of the proxy agent. (e.g: `DATA_PROXY_WS=http://127.0.0.1:8765`)
* execute your algorithm. it will connect to the servers through the proxy agent allowing you to execute multiple strategies

note: this env variable could be used to proxy the data websocket through a custom server too. 

## GoDoc

For a more in-depth look at the SDK, see the 
[GoDoc](https://godoc.org/github.com/market-development-strategy/alpaca-trade-api-go)
