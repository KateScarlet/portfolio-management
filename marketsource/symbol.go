package marketsource

import (
	"fmt"
	"strings"
)

// NormalizeSymbol converts any user input symbol to a canonical format.
// Format examples: 600519.SH, 000001.SZ, 00700.HK, AAPL.US, BTC.USD, 001811.CNOF, au9999.CN, GC.INTL
func NormalizeSymbol(symbol, market string) string {
	s := strings.ToUpper(strings.TrimSpace(symbol))

	// Strip common source-specific prefixes
	s = stripPrefixes(s)

	switch market {
	case "CN":
		return normalizeAShare(s)
	case "HK":
		return normalizeHK(s)
	case "US":
		return normalizeUS(s)
	case "CRYPTO":
		return normalizeCrypto(s)
	case "FUND":
		return normalizeFund(s)
	case "COMMODITY_CN":
		return normalizeCommodityCN(s)
	case "COMMODITY_INTL":
		return normalizeCommodityIntl(s)
	default:
		return s
	}
}

func stripPrefixes(s string) string {
	// sh/sz prefix (e.g., sh600519 -> 600519)
	if strings.HasPrefix(s, "SH") && len(s) > 2 {
		s = s[2:]
	} else if strings.HasPrefix(s, "SZ") && len(s) > 2 {
		s = s[2:]
	}
	// hk prefix (e.g., hk00700 -> 00700)
	if strings.HasPrefix(s, "HK") && len(s) > 2 {
		s = s[2:]
	}
	// gb_ prefix for US stocks (e.g., gb_aapl -> AAPL)
	if strings.HasPrefix(s, "GB_") {
		s = s[3:]
	}
	return s
}

func normalizeAShare(s string) string {
	// Already has exchange suffix
	if strings.HasSuffix(s, ".SH") || strings.HasSuffix(s, ".SZ") {
		return s
	}
	// 6-digit code: determine exchange by first digit
	if isDigitCode(s) {
		if s[0] == '6' || s[0] == '5' || s[0] == '7' || s[0] == '9' {
			return s + ".SH"
		}
		return s + ".SZ"
	}
	// Fallback
	return s + ".SH"
}

func normalizeHK(s string) string {
	if before, ok := strings.CutSuffix(s, ".HK"); ok {
		// Ensure 5-digit padding
		code := before
		for len(code) < 5 {
			code = "0" + code
		}
		return code + ".HK"
	}
	// Pad to 5 digits
	for len(s) < 5 {
		s = "0" + s
	}
	return s + ".HK"
}

func normalizeUS(s string) string {
	if strings.HasSuffix(s, ".US") {
		return s
	}
	return s + ".US"
}

func normalizeCrypto(s string) string {
	if strings.HasSuffix(s, ".CC") {
		return s
	}
	// Strip other suffixes if present
	s = strings.TrimSuffix(s, ".USDT")
	s = strings.TrimSuffix(s, ".US")
	s = strings.TrimSuffix(s, ".USD")
	// Handle Yahoo format: BTC-USD -> BTC
	s = strings.TrimSuffix(s, "-USD")
	s = strings.TrimSuffix(s, "-USDT")
	return s + ".CC"
}

func normalizeFund(s string) string {
	if strings.HasSuffix(s, ".CNOF") {
		return s
	}
	return s + ".CNOF"
}

func normalizeCommodityCN(s string) string {
	if strings.HasSuffix(s, ".CN") {
		return s
	}
	return s + ".CN"
}

func normalizeCommodityIntl(s string) string {
	if strings.HasSuffix(s, ".INTL") {
		return s
	}
	return s + ".INTL"
}

func isDigitCode(s string) bool {
	if len(s) != 6 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// NormalizeForSource converts a canonical symbol to the query format expected by a specific source.
func NormalizeForSource(symbol, market, source string) string {
	switch source {
	case "yahoo":
		return normalizeForYahoo(symbol, market)
	case "tencent":
		return normalizeForTencent(symbol, market)
	case "sina":
		return normalizeForSina(symbol, market)
	case "eastmoney":
		return normalizeForEastmoney(symbol, market)
	case "coingecko":
		return normalizeForCoingecko(symbol)
	default:
		return symbol
	}
}

func normalizeForYahoo(symbol, market string) string {
	switch market {
	case "CN":
		// 600519.SH -> 600519.SS, 000001.SZ -> 000001.SZ
		if before, ok := strings.CutSuffix(symbol, ".SH"); ok {
			return before + ".SS"
		}
		return symbol
	case "HK":
		// 00700.HK -> 00700.HK (unchanged)
		return symbol
	case "US":
		// AAPL.US -> AAPL (strip suffix)
		return strings.TrimSuffix(symbol, ".US")
	case "CRYPTO":
		// BTC.CC -> BTC-USD
		return strings.TrimSuffix(symbol, ".CC") + "-USD"
	case "FUND":
		// 001811.CNOF -> 001811.SZ (funds trade on Shenzhen exchange)
		code := strings.TrimSuffix(symbol, ".CNOF")
		return code + ".SZ"
	case "COMMODITY_INTL":
		// GC.INTL -> GC=F
		return strings.TrimSuffix(symbol, ".INTL") + "=F"
	default:
		return symbol
	}
}

func normalizeForTencent(symbol, market string) string {
	switch market {
	case "CN":
		code := strings.TrimSuffix(symbol, ".SH")
		code = strings.TrimSuffix(code, ".SZ")
		// Determine prefix from original suffix
		if strings.HasSuffix(symbol, ".SH") {
			return "sh" + code
		}
		return "sz" + code
	case "HK":
		code := strings.TrimSuffix(symbol, ".HK")
		for len(code) < 5 {
			code = "0" + code
		}
		return "hk" + code
	default:
		return symbol
	}
}

func normalizeForSina(symbol, market string) string {
	switch market {
	case "CN":
		code := strings.TrimSuffix(symbol, ".SH")
		code = strings.TrimSuffix(code, ".SZ")
		if strings.HasSuffix(symbol, ".SH") {
			return "sh" + code
		}
		return "sz" + code
	case "HK":
		code := strings.TrimSuffix(symbol, ".HK")
		for len(code) < 5 {
			code = "0" + code
		}
		return "hk" + code
	case "US":
		ticker := strings.TrimSuffix(symbol, ".US")
		return "gb_" + strings.ToLower(ticker)
	default:
		return symbol
	}
}

func normalizeForEastmoney(symbol, market string) string {
	switch market {
	case "CN":
		code := strings.TrimSuffix(symbol, ".SH")
		code = strings.TrimSuffix(code, ".SZ")
		if strings.HasSuffix(symbol, ".SH") {
			return "1." + code
		}
		return "0." + code
	case "US":
		ticker := strings.TrimSuffix(symbol, ".US")
		return "105." + ticker
	case "HK":
		code := strings.TrimSuffix(symbol, ".HK")
		return "116." + code
	case "FUND":
		code := strings.TrimSuffix(symbol, ".CNOF")
		return code
	case "COMMODITY_CN":
		code := strings.TrimSuffix(symbol, ".CN")
		return strings.ToLower(code)
	default:
		return symbol
	}
}

// coingeckoIDMap maps ticker symbols to CoinGecko IDs
var coingeckoIDMap = map[string]string{
	"BTC":   "bitcoin",
	"ETH":   "ethereum",
	"BNB":   "binancecoin",
	"SOL":   "solana",
	"XRP":   "ripple",
	"DOGE":  "dogecoin",
	"ADA":   "cardano",
	"DOT":   "polkadot",
	"AVAX":  "avalanche-2",
	"LINK":  "chainlink",
	"MATIC": "matic-network",
	"UNI":   "uniswap",
	"LTC":   "litecoin",
	"BCH":   "bitcoin-cash",
	"ATOM":  "cosmos",
	"FIL":   "filecoin",
	"ETC":   "ethereum-classic",
	"APE":   "apecoin",
	"ARB":   "arbitrum",
	"OP":    "optimism",
	"NEAR":  "near",
	"APT":   "aptos",
	"SUI":   "sui",
	"TRX":   "tron",
	"SHIB":  "shiba-inu",
	"PEPE":  "pepe",
}

func normalizeForCoingecko(symbol string) string {
	ticker := strings.TrimSuffix(symbol, ".CC")
	ticker = strings.TrimSuffix(ticker, ".USDT")
	ticker = strings.ToUpper(ticker)
	if id, ok := coingeckoIDMap[ticker]; ok {
		return id
	}
	return strings.ToLower(ticker)
}

// ExtractBaseSymbol removes the market suffix to get the bare code/ticker.
// e.g., "600519.SH" -> "600519", "AAPL.US" -> "AAPL"
func ExtractBaseSymbol(symbol string) string {
	if idx := strings.LastIndex(symbol, "."); idx > 0 {
		return symbol[:idx]
	}
	return symbol
}

// ValidateSymbol checks if a symbol matches the expected format for its market.
func ValidateSymbol(symbol, market string) error {
	suffix := getExpectedSuffix(market)
	if suffix == "" {
		return nil // no validation for unknown markets
	}
	if !strings.HasSuffix(symbol, suffix) {
		return fmt.Errorf("symbol %q does not match expected format for market %s (expected suffix %s)", symbol, market, suffix)
	}
	return nil
}

func getExpectedSuffix(market string) string {
	switch market {
	case "CN":
		return ".SH" // or .SZ, but we just check it ends with a valid suffix
	case "HK":
		return ".HK"
	case "US":
		return ".US"
	case "CRYPTO":
		return ".CC"
	case "FUND":
		return ".CNOF"
	case "COMMODITY_CN":
		return ".CN"
	case "COMMODITY_INTL":
		return ".INTL"
	default:
		return ""
	}
}
