package market

import (
	"context"
	"testing"
	"time"
)

// TestSinaProvider_GetQuote 测试获取实时行情
func TestSinaProvider_GetQuote(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	provider := NewSinaProvider()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 测试真实行情获取（需要网络）
	// 注意：实际使用时股票代码应从用户输入或Cogni上下文中获取
	// 这里仅用于测试新浪API的连通性
	testStock := "000001.SZ" // 平安银行，仅用于API测试
	quote, err := provider.GetQuote(ctx, testStock)
	if err != nil {
		t.Fatalf("get quote failed: %v", err)
	}

	t.Logf("Stock: %s", quote.Stock)
	t.Logf("Price: %.2f", quote.Price)
	t.Logf("Open: %.2f", quote.Open)
	t.Logf("High: %.2f", quote.High)
	t.Logf("Low: %.2f", quote.Low)
	t.Logf("Volume: %d", quote.Volume)
	t.Logf("Change: %.2f (%.2f%%)", quote.Change, quote.ChangePct*100)
	t.Logf("Timestamp: %s", quote.Timestamp.Format("2006-01-02 15:04:05"))

	// 验证数据合理性
	if quote.Price <= 0 {
		t.Error("price should be positive")
	}
	if quote.High < quote.Low {
		t.Error("high should be >= low")
	}
	if quote.Volume < 0 {
		t.Error("volume should be non-negative")
	}
}

// TestSinaProvider_GetBatchQuotes 测试批量获取行情
func TestSinaProvider_GetBatchQuotes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	provider := NewSinaProvider()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 测试批量获取行情（需要网络）
	// 注意：实际使用时股票代码应从用户输入或Cogni上下文中获取
	testStocks := []string{"000001.SZ", "600000.SH", "000002.SZ"} // 仅用于API测试
	quotes, err := provider.GetBatchQuotes(ctx, testStocks)
	if err != nil {
		t.Fatalf("get batch quotes failed: %v", err)
	}

	t.Logf("Got %d quotes", len(quotes))

	for stock, quote := range quotes {
		t.Logf("%s: %.2f (%.2f%%)", stock, quote.Price, quote.ChangePct*100)
	}

	if len(quotes) == 0 {
		t.Error("expected at least one quote")
	}
}

// TestSinaProvider_GetKLines 测试获取K线数据
func TestSinaProvider_GetKLines(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	provider := NewSinaProvider()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 测试获取K线数据（需要网络）
	// 注意：实际使用时股票代码应从用户输入或Cogni上下文中获取
	testStock := "000001.SZ" // 仅用于API测试
	klines, err := provider.GetKLines(ctx, testStock, "1d", 10)
	if err != nil {
		t.Fatalf("get klines failed: %v", err)
	}

	t.Logf("Got %d klines", len(klines))

	for i, kline := range klines {
		t.Logf("[%d] %s: O=%.2f H=%.2f L=%.2f C=%.2f V=%d",
			i, kline.Timestamp.Format("2006-01-02"),
			kline.Open, kline.High, kline.Low, kline.Close, kline.Volume)
	}

	if len(klines) == 0 {
		t.Error("expected at least one kline")
	}
}

// TestConvertToSinaCode 测试股票代码转换
func TestConvertToSinaCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"000001.SZ", "sz000001"},
		{"600000.SH", "sh600000"},
		{"000002.SZ", "sz000002"},
		{"601318.SH", "sh601318"},
	}

	for _, tt := range tests {
		result := convertToSinaCode(tt.input)
		if result != tt.expected {
			t.Errorf("convertToSinaCode(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}
