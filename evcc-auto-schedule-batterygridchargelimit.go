package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/joho/godotenv"
)

type EvccAPIRate struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Price float64   `json:"price"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

func main() {

	err := godotenv.Load()
	if err != nil {
		slog.Error("Error loading .env file")
		os.Exit(1)
	}

	apiRatesURL := os.Getenv("APIRATESURL")
	evccURL := os.Getenv("EVCCURL")
	now := time.Now()

	resp, err := httpClient.Get(apiRatesURL)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	var rates []EvccAPIRate
	err = json.Unmarshal(body, &rates)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	upcomingRates := make([]EvccAPIRate, 0)

	for _, r := range rates {
		if r.Start.After(now) {
			upcomingRates = append(upcomingRates, r)
		}
	}

	sort.Slice(upcomingRates, func(i, j int) bool {
		return upcomingRates[i].Price < upcomingRates[j].Price
	})

	// It takes approx ~4 hours to charge the battery, so find the 5th lowest price
	lowPrice := upcomingRates[4]

	// find the highest price _after_ lowPrice
	var highPrice EvccAPIRate
	for _, h := range upcomingRates {
		if h.Start.After(lowPrice.Start) && h.Price > highPrice.Price {
			highPrice = h
		}
	}

	fmt.Println("Low:", lowPrice.Price, "Start:", lowPrice.Start)
	fmt.Println("High:", highPrice.Price, "Start:", highPrice.Start)

	// Only schedule charge if highPrice is at least twice the lowprice
	var chargelimit float64 = 0
	if highPrice.Price > 2*lowPrice.Price {
		chargelimit = math.Ceil(lowPrice.Price*20) / 20
	}

	fmt.Println("Chargelimit:", chargelimit)

	url := fmt.Sprintf("%s%g", evccURL, chargelimit)

	postResp, err := httpClient.Post(url, "application/json", nil)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	defer postResp.Body.Close()

}
