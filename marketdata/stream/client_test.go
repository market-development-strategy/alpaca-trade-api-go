package stream

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	stocksTests = "stocks"
	cryptoTests = "crypto"
)

var tests = []struct {
	name string
}{
	{name: stocksTests},
	{name: cryptoTests},
}

func TestConnectFails(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connection := newMockConn()
			defer connection.close()
			connCreator := func(ctx context.Context, u url.URL) (conn, error) {
				return connection, nil
			}

			var c StreamClient
			switch tt.name {
			case stocksTests:
				c = NewStocksClient("iex",
					WithReconnectSettings(1, 0),
					withConnCreator(connCreator))
			case cryptoTests:
				c = NewCryptoClient(
					WithReconnectSettings(1, 0),
					withConnCreator(connCreator))
			}

			// server connection can not be established
			connection.readCh <- serializeToMsgpack(t, []map[string]interface{}{
				{
					"not": "good",
				},
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := c.Connect(ctx)

			assert.Error(t, err)
			assert.True(t, errors.Is(err, ErrNoConnected))
		})
	}
}

func TestConnectWithInvalidURL(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c StreamClient
			switch tt.name {
			case stocksTests:
				c = NewStocksClient("iex",
					WithBaseURL("http://192.168.0.%31/"),
					WithReconnectSettings(1, 0))
			case cryptoTests:
				c = NewCryptoClient(
					WithBaseURL("http://192.168.0.%31/"),
					WithReconnectSettings(1, 0))
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := c.Connect(ctx)

			assert.Error(t, err)
		})
	}
}

func TestConnectImmediatelyFailsInvalidCredentials(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connection := newMockConn()
			defer connection.close()
			connCreator := func(ctx context.Context, u url.URL) (conn, error) {
				return connection, nil
			}

			// if the error weren't irrecoverable then we would be retrying for quite a while
			// and the test would time out
			reconnectSettings := WithReconnectSettings(20, time.Second)

			var c StreamClient
			switch tt.name {
			case stocksTests:
				c = NewStocksClient("iex", reconnectSettings, withConnCreator(connCreator))
			case cryptoTests:
				c = NewCryptoClient(reconnectSettings, withConnCreator(connCreator))
			}

			// server welcomes the client
			connection.readCh <- serializeToMsgpack(t, []controlWithT{
				{
					Type: "success",
					Msg:  "connected",
				},
			})
			// server rejects the credentials
			connection.readCh <- serializeToMsgpack(t, []errorWithT{
				{
					Type: "error",
					Code: 402,
					Msg:  "auth failed",
				},
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := c.Connect(ctx)

			assert.Error(t, err)
			assert.True(t, errors.Is(err, ErrInvalidCredentials))
		})
	}
}

func TestContextCancelledBeforeConnect(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connection := newMockConn()
			defer connection.close()
			connCreator := func(ctx context.Context, u url.URL) (conn, error) {
				return connection, nil
			}

			var c StreamClient
			switch tt.name {
			case stocksTests:
				c = NewStocksClient("iex",
					WithBaseURL("http://test.paca/v2"),
					withConnCreator(connCreator))
			case cryptoTests:
				c = NewCryptoClient(
					WithBaseURL("http://test.paca/v2"),
					withConnCreator(connCreator))
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := c.Connect(ctx)
			assert.Error(t, err)
			assert.Error(t, <-c.Terminated())
		})
	}
}

func TestConnectSucceeds(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connection := newMockConn()
			defer connection.close()
			connCreator := func(ctx context.Context, u url.URL) (conn, error) {
				return connection, nil
			}

			writeInitialFlowMessagesToConn(t, connection, subscriptions{})

			var c StreamClient
			switch tt.name {
			case stocksTests:
				c = NewStocksClient("iex", withConnCreator(connCreator))
			case cryptoTests:
				c = NewCryptoClient(withConnCreator(connCreator))
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := c.Connect(ctx)
			require.NoError(t, err)

			// Connect can't be called multiple times
			err = c.Connect(ctx)
			assert.Equal(t, ErrConnectCalledMultipleTimes, err)
		})
	}
}

func TestSubscribeBeforeConnectStocks(t *testing.T) {
	c := NewStocksClient("iex")

	err := c.SubscribeToTrades(func(trade Trade) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToQuotes(func(quote Quote) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToBars(func(bar Bar) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToDailyBars(func(bar Bar) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromTrades()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromQuotes()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromBars()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromDailyBars()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
}

func TestSubscribeBeforeConnectCrypto(t *testing.T) {
	c := NewCryptoClient()

	err := c.SubscribeToTrades(func(trade CryptoTrade) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToQuotes(func(quote CryptoQuote) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToBars(func(bar CryptoBar) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToDailyBars(func(bar CryptoBar) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromTrades()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromQuotes()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromBars()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromDailyBars()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
}

func TestSubscribeMultipleCallsStocks(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{})

	c := NewStocksClient("iex", withConnCreator(func(ctx context.Context, u url.URL) (conn, error) {
		return connection, nil
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Connect(ctx)
	require.NoError(t, err)

	subErrCh := make(chan error, 2)
	subFunc := func() {
		subErrCh <- c.SubscribeToTrades(func(trade Trade) {}, "ALPACA")
	}

	// calling two Subscribes at the same time and also calling a sub change
	// without modifying symbols (should succeed immediately)
	go subFunc()
	err = c.SubscribeToTrades(func(trade Trade) {})
	assert.NoError(t, err)
	go subFunc()

	err = <-subErrCh
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrSubscriptionChangeAlreadyInProgress))
}

func TestSubscribeCalledButClientTerminatesCrypto(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{})

	c := NewCryptoClient(withConnCreator(func(ctx context.Context, u url.URL) (conn, error) {
		return connection, nil
	}))
	ctx, cancel := context.WithCancel(context.Background())

	err := c.Connect(ctx)
	require.NoError(t, err)

	checkInitialMessagesSentByClient(t, connection, "", "", c.(*cryptoClient).sub)
	subErrCh := make(chan error, 1)
	subFunc := func() {
		subErrCh <- c.SubscribeToTrades(func(trade CryptoTrade) {}, "PACOIN")
	}

	// calling Subscribe
	go subFunc()
	// making sure Subscribe got called
	subMsg := expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"PACOIN"}, subMsg["trades"])
	// terminating the client
	cancel()

	err = <-subErrCh
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrSubscriptionChangeInterrupted))

	// Subscribing after the client has terminated results in an error
	err = c.SubscribeToQuotes(func(quote CryptoQuote) {}, "BTCUSD", "ETCUSD")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrSubscriptionChangeAfterTerminated))
}

func TestSubscripitionAcrossConnectionIssues(t *testing.T) {
	conn1 := newMockConn()
	writeInitialFlowMessagesToConn(t, conn1, subscriptions{})

	key := "testkey"
	secret := "testsecret"
	c := NewStocksClient("iex",
		WithCredentials(key, secret),
		withConnCreator(func(ctx context.Context, u url.URL) (conn, error) {
			return conn1, nil
		}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// connect
	err := c.Connect(ctx)
	require.NoError(t, err)
	checkInitialMessagesSentByClient(t, conn1, key, secret, subscriptions{})

	// subscribing to something
	trades1 := []string{"AL", "PACA"}
	subRes := make(chan error)
	go func() {
		subRes <- c.SubscribeToTrades(func(trade Trade) {}, "AL", "PACA")
	}()
	sub := expectWrite(t, conn1)
	require.Equal(t, "subscribe", sub["action"])
	require.ElementsMatch(t, trades1, sub["trades"])

	// shutting down the first connection
	conn2 := newMockConn()
	writeInitialFlowMessagesToConn(t, conn2, subscriptions{})
	c.(*stocksClient).connCreator = func(ctx context.Context, u url.URL) (conn, error) {
		return conn2, nil
	}
	conn1.close()

	// checking whether the client sent what we wanted it to (auth,sub1,sub2)
	checkInitialMessagesSentByClient(t, conn2, key, secret, subscriptions{})
	sub = expectWrite(t, conn2)
	require.Equal(t, "subscribe", sub["action"])
	require.ElementsMatch(t, trades1, sub["trades"])

	// responding to the subscription request
	conn2.readCh <- serializeToMsgpack(t, []subWithT{
		{
			Type:   "subscription",
			Trades: trades1,
			Quotes: []string{},
			Bars:   []string{},
		},
	})
	require.NoError(t, <-subRes)
	require.ElementsMatch(t, trades1, c.(*stocksClient).sub.trades)

	// the connection is shut down and the new one isn't established for a while
	conn3 := newMockConn()
	defer conn3.close()
	c.(*stocksClient).connCreator = func(ctx context.Context, u url.URL) (conn, error) {
		time.Sleep(100 * time.Millisecond)
		writeInitialFlowMessagesToConn(t, conn3, subscriptions{trades: trades1})
		return conn3, nil
	}
	conn2.close()

	// call an unsubscribe with the connection being down
	unsubRes := make(chan error)
	go func() { unsubRes <- c.UnsubscribeFromTrades("AL") }()

	// connection starts up, proper messages (auth,sub,unsub)
	checkInitialMessagesSentByClient(t, conn3, key, secret, subscriptions{trades: trades1})
	unsub := expectWrite(t, conn3)
	require.Equal(t, "unsubscribe", unsub["action"])
	require.ElementsMatch(t, []string{"AL"}, unsub["trades"])

	// responding to the unsub request
	conn3.readCh <- serializeToMsgpack(t, []subWithT{
		{
			Type:   "subscription",
			Trades: []string{"PACA"},
			Quotes: []string{},
			Bars:   []string{},
		},
	})

	// make sure the sub has returned by now (client changed)
	require.NoError(t, <-unsubRes)
	require.ElementsMatch(t, []string{"PACA"}, c.(*stocksClient).sub.trades)
}

func TestSubscribeFailsDueToError(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{})

	c := NewCryptoClient(withConnCreator(func(ctx context.Context, u url.URL) (conn, error) {
		return connection, nil
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// connect
	err := c.Connect(ctx)
	require.NoError(t, err)
	checkInitialMessagesSentByClient(t, connection, "", "", subscriptions{})

	// attempting sub change
	subRes := make(chan error)
	subFunc := func() {
		subRes <- c.SubscribeToTrades(func(trade CryptoTrade) {}, "PACOIN")
	}
	go subFunc()
	// wait for message to be written
	subMsg := expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"PACOIN"}, subMsg["trades"])

	// sub change request fails
	connection.readCh <- serializeToMsgpack(t, []errorWithT{
		{
			Type: "error",
			Code: 405,
			Msg:  "symbol limit exceeded",
		},
	})

	// making sure the subscription request has failed
	err = <-subRes
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrSymbolLimitExceeded))

	// attempting another sub change
	go subFunc()
	// wait for message to be written
	subMsg = expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"PACOIN"}, subMsg["trades"])

	// sub change request interrupted by slow client
	connection.readCh <- serializeToMsgpack(t, []errorWithT{
		{
			Type: "error",
			Code: 407,
			Msg:  "slow client",
		},
	})

	// making sure the subscription request has failed
	err = <-subRes
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrSlowClient))
}

func TestPingFails(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connection := newMockConn()
			defer connection.close()
			connCreator := func(ctx context.Context, u url.URL) (conn, error) {
				return connection, nil
			}

			writeInitialFlowMessagesToConn(t, connection, subscriptions{})

			testTicker := newTestTicker()
			newPingTicker = func() ticker {
				return testTicker
			}

			var c StreamClient
			switch tt.name {
			case stocksTests:
				c = NewStocksClient("iex", WithReconnectSettings(1, 0), withConnCreator(connCreator))
			case cryptoTests:
				c = NewCryptoClient(WithReconnectSettings(1, 0), withConnCreator(connCreator))
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := c.Connect(ctx)
			require.NoError(t, err)

			// replacing connCreator with a new one that returns an error
			// so connection will not be reestablished
			connErr := errors.New("no connection")
			switch tt.name {
			case stocksTests:
				c.(*stocksClient).connCreator = func(ctx context.Context, u url.URL) (conn, error) {
					return nil, connErr
				}
			case cryptoTests:
				c.(*cryptoClient).connCreator = func(ctx context.Context, u url.URL) (conn, error) {
					return nil, connErr
				}
			}
			// disabling ping (but not closing the connection alltogether!)
			connection.pingDisabled = true
			// triggering a ping
			testTicker.Tick()

			err = <-c.Terminated()
			assert.Error(t, err)
			assert.True(t, errors.Is(err, connErr))
		})
	}
}

func TestCoreFunctionalityStocks(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{
		trades:    []string{"ALPACA"},
		quotes:    []string{"ALPACA"},
		bars:      []string{"ALPACA"},
		dailyBars: []string{"LPACA"},
		statuses:  []string{"ALPACA"},
	})

	trades := make(chan Trade, 10)
	quotes := make(chan Quote, 10)
	bars := make(chan Bar, 10)
	dailyBars := make(chan Bar, 10)
	tradingStatuses := make(chan TradingStatus, 10)
	c := NewStocksClient("iex",
		WithTrades(func(t Trade) { trades <- t }, "ALPACA"),
		WithQuotes(func(q Quote) { quotes <- q }, "ALPCA"),
		WithBars(func(b Bar) { bars <- b }, "ALPACA"),
		WithDailyBars(func(b Bar) { dailyBars <- b }, "LPACA"),
		WithStatuses(func(ts TradingStatus) { tradingStatuses <- ts }, "ALPACA"),
		withConnCreator(func(ctx context.Context, u url.URL) (conn, error) {
			return connection, nil
		}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// connecting with the client
	err := c.Connect(ctx)
	require.NoError(t, err)

	// sending two bars and a quote
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		barWithT{
			Type:   "b",
			Symbol: "ALPACA",
			Volume: 322,
		},
		barWithT{
			Type:   "d",
			Symbol: "LPACA",
			Open:   35.1,
			High:   36.2,
		},
		quoteWithT{
			Type:    "q",
			Symbol:  "ALPACA",
			BidSize: 42,
		},
	})
	// sending a trade
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		tradeWithT{
			Type:   "t",
			Symbol: "ALPACA",
			ID:     123,
		},
	})
	// sending a trading status
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		tradingStatusWithT{
			Type:       "s",
			Symbol:     "ALPACA",
			StatusCode: "H",
			ReasonCode: "T12",
			Tape:       "C",
		},
	})

	// checking contents
	select {
	case bar := <-bars:
		assert.EqualValues(t, 322, bar.Volume)
	case <-time.After(time.Second):
		require.Fail(t, "no bar received in time")
	}

	select {
	case dailyBar := <-dailyBars:
		assert.EqualValues(t, 35.1, dailyBar.Open)
		assert.EqualValues(t, 36.2, dailyBar.High)
	case <-time.After(time.Second):
		require.Fail(t, "no daily bar received in time")
	}

	select {
	case quote := <-quotes:
		assert.EqualValues(t, 42, quote.BidSize)
	case <-time.After(time.Second):
		require.Fail(t, "no quote received in time")
	}

	select {
	case trade := <-trades:
		assert.EqualValues(t, 123, trade.ID)
	case <-time.After(time.Second):
		require.Fail(t, "no trade received in time")
	}

	select {
	case ts := <-tradingStatuses:
		assert.Equal(t, "T12", ts.ReasonCode)
	case <-time.After(time.Second):
		require.Fail(t, "no trading status received in time")
	}
}

func TestCoreFunctionalityCrypto(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{
		trades:    []string{"BTCUSD"},
		quotes:    []string{"ETHUSD"},
		bars:      []string{"LTCUSD"},
		dailyBars: []string{"BCHUSD"},
	})

	trades := make(chan CryptoTrade, 10)
	quotes := make(chan CryptoQuote, 10)
	bars := make(chan CryptoBar, 10)
	dailyBars := make(chan CryptoBar, 10)
	c := NewCryptoClient(
		WithCryptoTrades(func(t CryptoTrade) { trades <- t }, "BTCUSD"),
		WithCryptoQuotes(func(q CryptoQuote) { quotes <- q }, "ETHUSD"),
		WithCryptoBars(func(b CryptoBar) { bars <- b }, "LTCUSD"),
		WithCryptoDailyBars(func(b CryptoBar) { dailyBars <- b }, "BCHUSD"),
		withConnCreator(func(ctx context.Context, u url.URL) (conn, error) {
			return connection, nil
		}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// connecting with the client
	err := c.Connect(ctx)
	require.NoError(t, err)

	// sending two bars and a quote
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		cryptoBarWithT{
			Type:   "b",
			Symbol: "LTCUSD",
			Volume: 10,
		},
		cryptoBarWithT{
			Type:   "d",
			Symbol: "LTCUSD",
			Open:   196.05,
			High:   196.3,
		},
		cryptoQuoteWithT{
			Type:     "q",
			Symbol:   "ETHUSD",
			AskPrice: 2848.53,
		},
	})
	// sending a trade
	ts := time.Date(2021, 6, 2, 15, 12, 4, 3534, time.UTC)
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		cryptoTradeWithT{
			Type:      "t",
			Symbol:    "BTCUSD",
			Timestamp: ts,
		},
	})

	// checking contents
	select {
	case bar := <-bars:
		assert.EqualValues(t, 10, bar.Volume)
	case <-time.After(time.Second):
		require.Fail(t, "no bar received in time")
	}

	select {
	case dailyBar := <-dailyBars:
		assert.EqualValues(t, 196.05, dailyBar.Open)
		assert.EqualValues(t, 196.3, dailyBar.High)
	case <-time.After(time.Second):
		require.Fail(t, "no daily bar received in time")
	}

	select {
	case quote := <-quotes:
		assert.Equal(t, "ETHUSD", quote.Symbol)
		assert.EqualValues(t, 2848.53, quote.AskPrice)
	case <-time.After(time.Second):
		require.Fail(t, "no quote received in time")
	}

	select {
	case trade := <-trades:
		assert.True(t, trade.Timestamp.Equal(ts))
	case <-time.After(time.Second):
		require.Fail(t, "no trade received in time")
	}
}

func writeInitialFlowMessagesToConn(
	t *testing.T, conn *mockConn, sub subscriptions,
) {
	// server welcomes the client
	conn.readCh <- serializeToMsgpack(t, []controlWithT{
		{
			Type: "success",
			Msg:  "connected",
		},
	})
	// server accepts authentication
	conn.readCh <- serializeToMsgpack(t, []controlWithT{
		{
			Type: "success",
			Msg:  "authenticated",
		},
	})

	if sub.noSubscribeCallNecessary() {
		return
	}

	// server accepts subscription
	conn.readCh <- serializeToMsgpack(t, []subWithT{
		{
			Type:      "subscription",
			Trades:    sub.trades,
			Quotes:    sub.quotes,
			Bars:      sub.bars,
			DailyBars: sub.dailyBars,
			Statuses:  sub.statuses,
		},
	})
}

func checkInitialMessagesSentByClient(
	t *testing.T, m *mockConn, key, secret string, sub subscriptions,
) {
	// auth
	auth := expectWrite(t, m)
	require.Equal(t, "auth", auth["action"])
	require.Equal(t, key, auth["key"])
	require.Equal(t, secret, auth["secret"])

	if sub.noSubscribeCallNecessary() {
		return
	}

	// subscribe
	s := expectWrite(t, m)
	require.Equal(t, "subscribe", s["action"])
	require.ElementsMatch(t, sub.trades, s["trades"])
	require.ElementsMatch(t, sub.quotes, s["quotes"])
	require.ElementsMatch(t, sub.bars, s["bars"])
	require.ElementsMatch(t, sub.dailyBars, s["dailyBars"])
	require.ElementsMatch(t, sub.statuses, s["statuses"])
}
