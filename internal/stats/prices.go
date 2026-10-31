package stats

import (
	"math"
	"regexp"
)

// PriceDate is the date of the embedded price table (R-STAT-001); every output names it.
const PriceDate = "2026-09-25"

// Price is USD per million tokens.
type Price struct {
	Input, Output, CacheWrite, CacheRead float64
}

// prices by exact model id: the Anthropic first-party rates on PriceDate (input, output and
// cache read as published; the 5-minute cache write at 1.25x input, the 1-hour one at 2x). A new
// version can cost less than its family, so an id not listed is unpriced. Verify against the
// pricing page when updating, and move PriceDate with it.
var prices = map[string]Price{
	"claude-fable-5-1":  {10, 50, 12.5, 0.25},
	"claude-fable-5":    {10, 50, 12.5, 1},
	"claude-opus-5-5":   {4, 20, 5, 0.20},
	"claude-opus-5":     {5, 25, 6.25, 0.5},
	"claude-opus-4-1":   {15, 75, 18.75, 1.5},
	"claude-sonnet-5-5": {2, 10, 2.5, 0.10},
	"claude-sonnet-5":   {2, 10, 2.5, 0.20},
	"claude-sonnet-4-6": {3, 15, 3.75, 0.3},
	"claude-haiku-4-5":  {1, 5, 1.25, 0.1},
}

var dateSuffix = regexp.MustCompile(`-\d{8}$`)

// Cost prices u for model in whole cents (R-STAT-001): a trailing -YYYYMMDD is stripped, then the
// id must be in the table. priced is false otherwise; its cost is 0 and the caller says "unpriced".
func Cost(model string, u Usage) (cents int, priced bool) {
	p, ok := prices[dateSuffix.ReplaceAllString(model, "")]
	if !ok {
		return 0, false
	}
	w5 := u.CacheWrite - u.CacheWrite1h
	usd := (float64(u.Input)*p.Input + float64(u.Output)*p.Output + float64(w5)*p.CacheWrite +
		float64(u.CacheWrite1h)*2*p.Input + float64(u.CacheRead)*p.CacheRead) / 1e6
	return int(math.Round(usd * 100)), true
}
