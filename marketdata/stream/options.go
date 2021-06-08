package stream

import (
	"context"
	"net/url"
	"os"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v2/common"
)

// StockOption is a configuration option for the StockClient
type StockOption interface {
	applyStock(*stockOptions)
}

// CryptoOption is a configuration option for the CryptoClient
type CryptoOption interface {
	applyCrypto(*cryptoOptions)
}

// Option is a configuration option that can be used for both StockClient and CryptoClient
type Option interface {
	StockOption
	CryptoOption
}

type options struct {
	logger         Logger
	baseURL        string
	key            string
	secret         string
	reconnectLimit int
	reconnectDelay time.Duration
	processorCount int
	bufferSize     int
	trades         []string
	quotes         []string
	bars           []string
	dailyBars      []string

	// for testing only
	connCreator func(ctx context.Context, u url.URL) (conn, error)
}

type funcOption struct {
	f func(*options)
}

func (fo *funcOption) applyCrypto(o *cryptoOptions) {
	fo.f(&o.options)
}

func (fo *funcOption) applyStock(o *stockOptions) {
	fo.f(&o.options)
}

func newFuncOption(f func(*options)) *funcOption {
	return &funcOption{
		f: f,
	}
}

// WithLogger configures the logger
func WithLogger(logger Logger) Option {
	return newFuncOption(func(o *options) {
		o.logger = logger
	})
}

// WithBaseURL configures the base URL
func WithBaseURL(url string) Option {
	return newFuncOption(func(o *options) {
		o.baseURL = url
	})
}

// WithCredentials configures the key and secret to use
func WithCredentials(key, secret string) Option {
	return newFuncOption(func(o *options) {
		o.key = key
		o.secret = secret
	})
}

// WithReconnectSettings configures how many consecutive connection
// errors should be accepted and the delay (that is multipled by the number of consecutive errors)
// between retries. limit = 0 means the client will try restarting indefinitely unless it runs into
// an irrecoverable error (such as invalid credentials).
func WithReconnectSettings(limit int, delay time.Duration) Option {
	return newFuncOption(func(o *options) {
		o.reconnectLimit = limit
		o.reconnectDelay = delay
	})
}

// WithProcessors configures how many goroutines should process incoming
// messages. Increasing this past 1 means that the order of processing is not
// necessarily the same as the order of arrival the from server.
func WithProcessors(count int) Option {
	return newFuncOption(func(o *options) {
		o.processorCount = count
	})
}

// WithBufferSize sets the size for the buffer that is used for messages received
// from the server
func WithBufferSize(size int) Option {
	return newFuncOption(func(o *options) {
		o.bufferSize = size
	})
}

func withConnCreator(connCreator func(ctx context.Context, u url.URL) (conn, error)) Option {
	return newFuncOption(func(o *options) {
		o.connCreator = connCreator
	})
}

type stockOptions struct {
	options
	tradeHandler         func(Trade)
	quoteHandler         func(Quote)
	barHandler           func(Bar)
	dailyBarHandler      func(Bar)
	tradingStatusHandler func(TradingStatus)
}

// defaultStockOptions are the default options for a client.
// Don't change this in a backward incompatible way!
func defaultStockOptions() *stockOptions {
	baseURL := "https://stream.data.alpaca.markets/v2"
	if s := os.Getenv("DATA_PROXY_WS"); s != "" {
		baseURL = s
	}

	return &stockOptions{
		options: options{
			logger:         newStdLog(),
			baseURL:        baseURL,
			key:            common.Credentials().ID,
			secret:         common.Credentials().Secret,
			reconnectLimit: 20,
			reconnectDelay: 150 * time.Millisecond,
			processorCount: 1,
			bufferSize:     100000,
			trades:         []string{},
			quotes:         []string{},
			bars:           []string{},
			dailyBars:      []string{},
			connCreator: func(ctx context.Context, u url.URL) (conn, error) {
				return newNhooyrWebsocketConn(ctx, u)
			},
		},
		tradeHandler:         func(t Trade) {},
		quoteHandler:         func(q Quote) {},
		barHandler:           func(b Bar) {},
		dailyBarHandler:      func(b Bar) {},
		tradingStatusHandler: func(ts TradingStatus) {},
	}
}

func (o *stockOptions) applyStock(opts ...StockOption) {
	for _, opt := range opts {
		opt.applyStock(o)
	}
}

type funcStockOption struct {
	f func(*stockOptions)
}

func (fo *funcStockOption) applyStock(o *stockOptions) {
	fo.f(o)
}

func newFuncStockOption(f func(*stockOptions)) StockOption {
	return &funcStockOption{
		f: f,
	}
}

// WithTrades configures inital trade symbols to subscribe to and the handler
func WithTrades(handler func(Trade), symbols ...string) StockOption {
	return newFuncStockOption(func(o *stockOptions) {
		o.trades = symbols
		o.tradeHandler = handler
	})
}

// WithQuotes configures inital quote symbols to subscribe to and the handler
func WithQuotes(handler func(Quote), symbols ...string) StockOption {
	return newFuncStockOption(func(o *stockOptions) {
		o.quotes = symbols
		o.quoteHandler = handler
	})
}

// WithBars configures inital bar symbols to subscribe to and the handler
func WithBars(handler func(Bar), symbols ...string) StockOption {
	return newFuncStockOption(func(o *stockOptions) {
		o.bars = symbols
		o.barHandler = handler
	})
}

// WithDailyBars configures inital daily bar symbols to subscribe to and the handler
func WithDailyBars(handler func(Bar), symbols ...string) StockOption {
	return newFuncStockOption(func(o *stockOptions) {
		o.dailyBars = symbols
		o.dailyBarHandler = handler
	})
}

// WithTradingStatusHandler configures the trading status handler
func WithTradingStatusHandler(handler func(TradingStatus)) StockOption {
	return newFuncStockOption(func(o *stockOptions) {
		o.tradingStatusHandler = handler
	})
}

type cryptoOptions struct {
	options
	tradeHandler    func(CryptoTrade)
	quoteHandler    func(CryptoQuote)
	barHandler      func(CryptoBar)
	dailyBarHandler func(CryptoBar)
}

// defaultCryptoOptions are the default options for a client.
// Don't change this in a backward incompatible way!
func defaultCryptoOptions() *cryptoOptions {
	baseURL := "https://stream.data.alpaca.markets/crypto"
	if s := os.Getenv("DATA_PROXY_WS"); s != "" {
		baseURL = s
	}

	return &cryptoOptions{
		options: options{
			logger:         newStdLog(),
			baseURL:        baseURL,
			key:            common.Credentials().ID,
			secret:         common.Credentials().Secret,
			reconnectLimit: 20,
			reconnectDelay: 150 * time.Millisecond,
			processorCount: 1,
			bufferSize:     100000,
			trades:         []string{},
			quotes:         []string{},
			bars:           []string{},
			dailyBars:      []string{},
			connCreator: func(ctx context.Context, u url.URL) (conn, error) {
				return newNhooyrWebsocketConn(ctx, u)
			},
		},
		tradeHandler:    func(t CryptoTrade) {},
		quoteHandler:    func(q CryptoQuote) {},
		barHandler:      func(b CryptoBar) {},
		dailyBarHandler: func(b CryptoBar) {},
	}
}

func (o *cryptoOptions) applyCrypto(opts ...CryptoOption) {
	for _, opt := range opts {
		opt.applyCrypto(o)
	}
}

type funcCryptoOption struct {
	f func(*cryptoOptions)
}

func (fo *funcCryptoOption) applyCrypto(o *cryptoOptions) {
	fo.f(o)
}

func newFuncCryptoOption(f func(*cryptoOptions)) *funcCryptoOption {
	return &funcCryptoOption{
		f: f,
	}
}

// WithCryptoTrades configures inital trade symbols to subscribe to and the handler
func WithCryptoTrades(handler func(CryptoTrade), symbols ...string) CryptoOption {
	return newFuncCryptoOption(func(o *cryptoOptions) {
		o.trades = symbols
		o.tradeHandler = handler
	})
}

// WithCryptoQuotes configures inital quote symbols to subscribe to and the handler
func WithCryptoQuotes(handler func(CryptoQuote), symbols ...string) CryptoOption {
	return newFuncCryptoOption(func(o *cryptoOptions) {
		o.quotes = symbols
		o.quoteHandler = handler
	})
}

// WithCryptoBars configures inital bar symbols to subscribe to and the handler
func WithCryptoBars(handler func(CryptoBar), symbols ...string) CryptoOption {
	return newFuncCryptoOption(func(o *cryptoOptions) {
		o.bars = symbols
		o.barHandler = handler
	})
}

// WithCryptoDailyBars configures inital daily bar symbols to subscribe to and the handler
func WithCryptoDailyBars(handler func(CryptoBar), symbols ...string) CryptoOption {
	return newFuncCryptoOption(func(o *cryptoOptions) {
		o.dailyBars = symbols
		o.dailyBarHandler = handler
	})
}