package main

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/market-development-strategy/alpaca-trade-api-go/alpaca"
	"github.com/market-development-strategy/alpaca-trade-api-go/common"
	"github.com/market-development-strategy/alpaca-trade-api-go/polygon"
	"github.com/market-development-strategy/alpaca-trade-api-go/stream"
	"github.com/shopspring/decimal"
)

type alpacaClientContainer struct {
	client        *alpaca.Client
	tickSize      int
	tickIndex     int
	baseBet       float64
	currStreak    streak
	currOrder     string
	lastPrice     float64
	lastTradeTime time.Time
	stock         string
	position      int64
	equity        float64
	marginMult    float64
	seconds       int
}

type streak struct {
	start      float64
	count      int
	increasing bool
}

var alpacaClient alpacaClientContainer

// The MartingaleTrader bets that streaks of increases or decreases in a stock's
// price are likely to break, and increases its bet each time it is wrong.
func init() {
	API_KEY := "YOUR_API_KEY_HERE"
	API_SECRET := "YOUR_API_SECRET_HERE"
	BASE_URL := "https://paper-api.alpaca.markets"

	// Check for environment variables
	if common.Credentials().ID == "" {
		os.Setenv(common.EnvApiKeyID, API_KEY)
	}
	if common.Credentials().Secret == "" {
		os.Setenv(common.EnvApiSecretKey, API_SECRET)
	}
	// os.Setenv("APCA_API_VERSION", "v1")
	alpaca.SetBaseUrl(BASE_URL)

	// Check if user input a stock, default is SPY
	stock := "AAPL"
	if len(os.Args[1:]) == 1 {
		stock = os.Args[1]
	}

	client := alpaca.NewClient(common.Credentials())

	// Cancel any open orders so they don't interfere with this script
	client.CancelAllOrders()

	pos, err := client.GetPosition(stock)
	position := int64(0)
	if err != nil {
		// No position exists
	} else {
		position = pos.Qty.IntPart()
	}

	// Figure out how much money we have to work with, accounting for margin
	accountInfo, err := client.GetAccount()
	if err != nil {
		panic(err)
	}

	equity, _ := accountInfo.Equity.Float64()
	marginMult, err := strconv.ParseFloat(accountInfo.Multiplier, 64)
	if err != nil {
		panic(err)
	}

	totalBuyingPower := marginMult * equity
	fmt.Printf("Initial total buying power = %.2f\n", totalBuyingPower)

	alpacaClient = alpacaClientContainer{
		client,
		5,
		4,
		.1,
		streak{
			0,
			0,
			true,
		},
		"",
		0,
		time.Now().UTC(),
		stock,
		position,
		equity,
		marginMult,
		0,
	}
}

func main() {
	USE_POLYGON := false

	// First, cancel any existing orders so they don't impact our buying power.
	status, until, limit := "open", time.Now(), 100
	orders, _ := alpacaClient.client.ListOrders(&status, &until, &limit, nil)
	for _, order := range orders {
		_ = alpacaClient.client.CancelOrder(order.ID)
	}

	if USE_POLYGON {
		stream.SetDataStream("polygon")
		if err := stream.Register(fmt.Sprintf("A.%s", alpacaClient.stock), handleAggs); err != nil {
			panic(err)
		}
	} else {
		if err := stream.Register(fmt.Sprintf("T.%s", alpacaClient.stock), handleAlpacaAggs); err != nil {
			panic(err)
		}
	}

	if err := stream.Register("trade_updates", handleTrades); err != nil {
		panic(err)
	}

	select {}
}

// Listen for second aggregates and perform trading logic
func handleAggs(msg interface{}) {
	data := msg.(polygon.StreamAggregate)

	if data.Symbol != alpacaClient.stock {
		return
	}

	alpacaClient.tickIndex = (alpacaClient.tickIndex + 1) % alpacaClient.tickSize
	if alpacaClient.tickIndex == 0 {
		// It's time to update

		// Update price info
		tickOpen := alpacaClient.lastPrice
		tickClose := float64(data.ClosePrice)
		alpacaClient.lastPrice = tickClose

		alpacaClient.processTick(tickOpen, tickClose)
	}
}

// Listen for quote data and perform trading logic
func handleAlpacaAggs(msg interface{}) {
	data := msg.(alpaca.StreamTrade)

	if data.Symbol != alpacaClient.stock {
		return
	}

	now := time.Now().UTC()
	if now.Sub(alpacaClient.lastTradeTime) < time.Second {
		// don't react every tick unless at least 1 second past
		return
	}
	alpacaClient.lastTradeTime = now

	alpacaClient.tickIndex = (alpacaClient.tickIndex + 1) % alpacaClient.tickSize
	if alpacaClient.tickIndex == 0 {
		// It's time to update

		// Update price info
		tickOpen := alpacaClient.lastPrice
		tickClose := float64(data.Price)
		alpacaClient.lastPrice = tickClose

		alpacaClient.processTick(tickOpen, tickClose)
	}
}

// Listen for updates to our orders
func handleTrades(msg interface{}) {
	data := msg.(alpaca.TradeUpdate)
	fmt.Printf("%s event received for order %s.\n", data.Event, data.Order.ID)

	if data.Order.Symbol != alpacaClient.stock {
		// The order was for a position unrelated to this script
		return
	}

	eventType := data.Event
	oid := data.Order.ID

	if eventType == "fill" || eventType == "partial_fill" {
		// Our position size has changed
		pos, err := alpacaClient.client.GetPosition(alpacaClient.stock)
		if err != nil {
			alpacaClient.position = 0
		} else {
			alpacaClient.position = pos.Qty.IntPart()
		}

		fmt.Printf("New position size due to order fill: %d\n", alpacaClient.position)
		if eventType == "fill" && alpacaClient.currOrder == oid {
			alpacaClient.currOrder = ""
		}
	} else if eventType == "rejected" || eventType == "canceled" {
		if alpacaClient.currOrder == oid {
			// Our last order should be removed
			alpacaClient.currOrder = ""
		}
	} else if eventType == "new" {
		alpacaClient.currOrder = oid
	} else {
		fmt.Printf("Unexpected order event type %s received\n", eventType)
	}
}

func (alp alpacaClientContainer) processTick(tickOpen float64, tickClose float64) {
	// Update streak info
	diff := tickClose - tickOpen
	if math.Abs(diff) >= .01 {
		// There was a meaningful change in the price
		alp.currStreak.count++
		increasing := tickOpen > tickClose
		if alp.currStreak.increasing != increasing {
			// It moved in the opposite direction of the streak.
			// Therefore, the streak is over, and we should reset.

			// Empty out the position
			if alp.position != 0 {
				_, err := alp.sendOrder(0)
				if err != nil {
					panic(err)
				}
			}

			// Reset variables
			alp.currStreak.increasing = increasing
			alp.currStreak.start = tickOpen
			alp.currStreak.count = 0
		} else {
			// Calculate the number of shares we want to be holding
			totalBuyingPower := alp.equity * alp.marginMult
			targetValue := math.Pow(2, float64(alp.currStreak.count)) * alp.baseBet * totalBuyingPower
			if targetValue > totalBuyingPower {
				// Limit the amount we can buy to a bit (1 share)
				// less than our total buying power
				targetValue = totalBuyingPower - alp.lastPrice
			}
			targetQty := int(targetValue / alp.lastPrice)
			if alp.currStreak.increasing {
				targetQty = -targetQty
			}

			// We don't want to have two orders open at once
			if int64(targetQty)-alp.position != 0 {
				if alpacaClient.currOrder != "" {
					err := alp.client.CancelOrder(alpacaClient.currOrder)

					if err != nil {
						panic(err)
					}

					alpacaClient.currOrder = ""
				}

				_, err := alp.sendOrder(targetQty)

				if err != nil {
					panic(err)
				}
			}
		}
	}

	// Update our account balance
	acct, err := alp.client.GetAccount()
	if err != nil {
		panic(err)
	}

	alp.equity, _ = acct.Equity.Float64()
}

func (alp alpacaClientContainer) sendOrder(targetQty int) (string, error) {
	delta := float64(int64(targetQty) - alp.position)

	fmt.Printf("Ordering towards %d...\n", targetQty)

	qty := float64(0)
	side := alpaca.Side("")

	if delta > 0 {
		side = alpaca.Buy
		qty = delta
		if alp.position < 0 {
			qty = math.Min(math.Abs(float64(alp.position)), qty)
		}
		fmt.Printf("Buying %d shares.\n", int64(qty))

	} else if delta < 0 {
		side = alpaca.Sell
		qty = math.Abs(delta)
		if alp.position > 0 {
			qty = math.Min(math.Abs(float64(alp.position)), qty)
		}
		fmt.Printf("Selling %d shares.\n", int64(qty))
	}

	// Follow [L] instructions to use limit orders
	if qty > 0 {
		account, _ := alp.client.GetAccount()

		// [L] Uncomment line below
		limitPrice := decimal.NewFromFloat(alp.lastPrice)

		alp.currOrder = randomString()
		alp.client.PlaceOrder(alpaca.PlaceOrderRequest{
			AccountID: account.ID,
			AssetKey:  &alp.stock,
			Qty:       decimal.NewFromFloat(qty),
			Side:      side,
			Type:      alpaca.Limit, // [L] Change to alpaca.Limit
			// [L] Uncomment line below
			LimitPrice:    &limitPrice,
			TimeInForce:   alpaca.Day,
			ClientOrderID: alp.currOrder,
		})

		return alp.currOrder, nil
	}

	return "", errors.New("Non-positive quantity given")
}

func randomString() string {
	rand.Seed(time.Now().Unix())
	characters := "abcdefghijklmnopqrstuvwxyz"
	resSize := 10

	var output strings.Builder

	for i := 0; i < resSize; i++ {
		index := rand.Intn(len(characters))
		output.WriteString(string(characters[index]))
	}
	return output.String()
}
