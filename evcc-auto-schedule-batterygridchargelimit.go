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

type RatesResponse struct {
	Result Result `json:"result"`
}

type Result struct {
	Rates []Rates `json:"rates"`
}

type Rates struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Price float64   `json:"price"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

func main() {

	err := godotenv.Load()
	if err != nil {
		slog.Warn("Error loading .env file")
	}

	evcc := os.Getenv("EVCC")

	if evcc == "" {
		slog.Error("missing EVCC env")
		os.Exit(1)
	}

	ratesURL := fmt.Sprintf("%s/api/tariff/grid", evcc)
	chargelimitURL := fmt.Sprintf("%s/api/batterygridchargelimit", evcc)

	resp, err := httpClient.Get(ratesURL)
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

	var ratesResponse RatesResponse

	err = json.Unmarshal(body, &ratesResponse)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	rates := ratesResponse.Result.Rates

	if len(rates) < 4 {
		slog.Error("not enough rates to do anything meaningful")
		os.Exit(1)
	}

	sort.Slice(rates, func(i, j int) bool {
		return rates[i].Price < rates[j].Price
	})

	// It takes approx ~4 hours to charge the battery, so find the 4th lowest price
	lowPrice := rates[3]

	// find the highest price _after_ lowPrice
	var highPrice Rates
	for _, h := range rates {
		if h.Start.After(lowPrice.Start) && h.Price > highPrice.Price {
			highPrice = h
		}
	}

	fmt.Println("Low:", lowPrice.Price, "Start:", lowPrice.Start)
	fmt.Println("High:", highPrice.Price, "Start:", highPrice.Start)

	diff := highPrice.Price - lowPrice.Price
	fmt.Println("Diff:", diff)

	// Only set chargelimit if highPrice is at least 2DKK higher than lowPrice. otherwise use charge if the price is totally negative (as if it would ever happen...)
	var chargelimit float64 = 0
	if diff > 2 {
		chargelimit = math.Ceil(lowPrice.Price*20) / 20
	}

	fmt.Println("Chargelimit:", chargelimit)

	url := fmt.Sprintf("%s/%g", chargelimitURL, chargelimit)
	fmt.Println(url)

	postResp, err := httpClient.Post(url, "application/json", nil)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	defer postResp.Body.Close()

	p, err := io.ReadAll(postResp.Body)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	fmt.Println(string(p))

}
