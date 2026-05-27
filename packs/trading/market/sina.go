package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/trading"
)

// ──────────────────────────────────────────────
// MarketDataProvider — 实时行情数据提供者
// 使用新浪财经、腾讯财经等公开 API
// ──────────────────────────────────────────────

// Provider 行情数据提供者接口
type Provider interface {
	// GetQuote 获取实时行情
	GetQuote(ctx context.Context, stock string) (*trading.Quote, error)

	// GetKLines 获取K线数据
	GetKLines(ctx context.Context, stock string, period string, count int) ([]trading.KLine, error)

	// GetBatchQuotes 批量获取行情
	GetBatchQuotes(ctx context.Context, stocks []string) (map[string]*trading.Quote, error)
}

// ──────────────────────────────────────────────
// SinaProvider — 新浪财经数据源
// 免费、稳定、实时性好
// ──────────────────────────────────────────────

// SinaProvider 新浪财经数据提供者
type SinaProvider struct {
	client *http.Client
}

// NewSinaProvider 创建新浪财经数据提供者
func NewSinaProvider() *SinaProvider {
	return &SinaProvider{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetQuote 获取实时行情
func (p *SinaProvider) GetQuote(ctx context.Context, stock string) (*trading.Quote, error) {
	// 转换股票代码格式
	// 000001.SZ -> sz000001
	// 600000.SH -> sh600000
	sinaCode := convertToSinaCode(stock)

	url := fmt.Sprintf("https://hq.sinajs.cn/list=%s", sinaCode)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Referer", "https://finance.sina.com.cn")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// 解析新浪返回的数据
	// var hq_str_sz000001="平安银行,10.37,10.38,10.41,10.44,10.36,10.40,10.41,123456789,1234567890.00,..."
	quote, err := parseSinaQuote(string(body), stock)
	if err != nil {
		return nil, fmt.Errorf("parse quote: %w", err)
	}

	return quote, nil
}

// GetKLines 获取K线数据
func (p *SinaProvider) GetKLines(ctx context.Context, stock string, period string, count int) ([]trading.KLine, error) {
	// 新浪K线接口
	// http://money.finance.sina.com.cn/quotes_service/api/json_v2.php/CN_MarketData.getKLineData?symbol=sz000001&scale=240&ma=5&datalen=100

	sinaCode := convertToSinaCode(stock)

	// 转换周期
	scale := convertPeriodToScale(period)

	url := fmt.Sprintf("http://money.finance.sina.com.cn/quotes_service/api/json_v2.php/CN_MarketData.getKLineData?symbol=%s&scale=%d&datalen=%d",
		sinaCode, scale, count)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// 解析K线数据
	klines, err := parseSinaKLines(body, stock)
	if err != nil {
		return nil, fmt.Errorf("parse klines: %w", err)
	}

	return klines, nil
}

// GetBatchQuotes 批量获取行情
func (p *SinaProvider) GetBatchQuotes(ctx context.Context, stocks []string) (map[string]*trading.Quote, error) {
	if len(stocks) == 0 {
		return make(map[string]*trading.Quote), nil
	}

	// 转换股票代码
	sinaCodes := make([]string, len(stocks))
	for i, stock := range stocks {
		sinaCodes[i] = convertToSinaCode(stock)
	}

	// 批量请求（最多一次50个）
	url := fmt.Sprintf("https://hq.sinajs.cn/list=%s", strings.Join(sinaCodes, ","))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Referer", "https://finance.sina.com.cn")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// 解析批量行情
	quotes := make(map[string]*trading.Quote)
	lines := strings.Split(string(body), "\n")

	for i, line := range lines {
		if i >= len(stocks) {
			break
		}
		if strings.TrimSpace(line) == "" {
			continue
		}

		quote, err := parseSinaQuote(line, stocks[i])
		if err != nil {
			continue
		}
		quotes[stocks[i]] = quote
	}

	return quotes, nil
}

// convertToSinaCode 转换股票代码格式
func convertToSinaCode(stock string) string {
	// 000001.SZ -> sz000001
	// 600000.SH -> sh600000
	parts := strings.Split(stock, ".")
	if len(parts) != 2 {
		return stock
	}

	code := parts[0]
	market := strings.ToLower(parts[1])

	return market + code
}

// parseSinaQuote 解析新浪行情数据
func parseSinaQuote(data string, stock string) (*trading.Quote, error) {
	// var hq_str_sz000001="平安银行,10.37,10.38,10.41,10.44,10.36,10.40,10.41,123456789,1234567890.00,..."

	start := strings.Index(data, "\"")
	end := strings.LastIndex(data, "\"")
	if start == -1 || end == -1 || start >= end {
		return nil, fmt.Errorf("invalid data format")
	}

	content := data[start+1 : end]
	fields := strings.Split(content, ",")

	if len(fields) < 32 {
		return nil, fmt.Errorf("insufficient fields: %d", len(fields))
	}

	// 字段说明：
	// 0: 股票名称
	// 1: 今日开盘价
	// 2: 昨日收盘价
	// 3: 当前价格
	// 4: 今日最高价
	// 5: 今日最低价
	// 6: 竞买价
	// 7: 竞卖价
	// 8: 成交量（股）
	// 9: 成交额（元）

	price, _ := strconv.ParseFloat(fields[3], 64)
	open, _ := strconv.ParseFloat(fields[1], 64)
	high, _ := strconv.ParseFloat(fields[4], 64)
	low, _ := strconv.ParseFloat(fields[5], 64)
	prevClose, _ := strconv.ParseFloat(fields[2], 64)
	volume, _ := strconv.ParseInt(fields[8], 10, 64)

	change := price - prevClose
	changePct := 0.0
	if prevClose > 0 {
		changePct = change / prevClose
	}

	return &trading.Quote{
		Stock:     stock,
		Price:     price,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     price,
		Volume:    volume,
		Timestamp: time.Now(),
		Change:    change,
		ChangePct: changePct,
	}, nil
}

// parseSinaKLines 解析新浪K线数据
func parseSinaKLines(data []byte, stock string) ([]trading.KLine, error) {
	// JSON格式：[{"day":"2024-01-01","open":"10.00","high":"10.50","low":"9.80","close":"10.20","volume":"1000000"}]

	var rawKLines []struct {
		Day    string `json:"day"`
		Open   string `json:"open"`
		High   string `json:"high"`
		Low    string `json:"low"`
		Close  string `json:"close"`
		Volume string `json:"volume"`
	}

	if err := json.Unmarshal(data, &rawKLines); err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}

	klines := make([]trading.KLine, 0, len(rawKLines))
	for _, raw := range rawKLines {
		timestamp, _ := time.Parse("2006-01-02", raw.Day)
		open, _ := strconv.ParseFloat(raw.Open, 64)
		high, _ := strconv.ParseFloat(raw.High, 64)
		low, _ := strconv.ParseFloat(raw.Low, 64)
		close, _ := strconv.ParseFloat(raw.Close, 64)
		volume, _ := strconv.ParseInt(raw.Volume, 10, 64)

		klines = append(klines, trading.KLine{
			Stock:     stock,
			Timestamp: timestamp,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			Amount:    close * float64(volume),
		})
	}

	return klines, nil
}

// convertPeriodToScale 转换周期到新浪的scale参数
func convertPeriodToScale(period string) int {
	switch period {
	case "1m":
		return 1
	case "5m":
		return 5
	case "15m":
		return 15
	case "30m":
		return 30
	case "1h":
		return 60
	case "1d":
		return 240
	case "1w":
		return 1440
	default:
		return 240 // 默认日线
	}
}
