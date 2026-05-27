package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/trading"
	"yunque-agent/packs/trading/market"
)

// ──────────────────────────────────────────────
// MarketMonitor — 盯盘监控系统
// 实时监控股票价格、触发预警、自动分析
// ──────────────────────────────────────────────

// AlertType 预警类型
type AlertType string

const (
	AlertPriceUp       AlertType = "price_up"        // 价格上涨
	AlertPriceDown     AlertType = "price_down"      // 价格下跌
	AlertBreakHigh     AlertType = "break_high"      // 突破新高
	AlertBreakLow      AlertType = "break_low"       // 跌破新低
	AlertVolumeSpike   AlertType = "volume_spike"    // 成交量放大
	AlertLimitUp       AlertType = "limit_up"        // 涨停
	AlertLimitDown     AlertType = "limit_down"      // 跌停
	AlertStrategySignal AlertType = "strategy_signal" // 策略信号
)

// Alert 预警信息
type Alert struct {
	Type      AlertType          `json:"type"`
	Stock     string             `json:"stock"`
	Message   string             `json:"message"`
	Price     float64            `json:"price"`
	ChangePct float64            `json:"change_pct"`
	Timestamp time.Time          `json:"timestamp"`
	Data      map[string]any     `json:"data,omitempty"`
}

// WatchItem 监控项
type WatchItem struct {
	Stock         string    `json:"stock"`
	Name          string    `json:"name"`
	AddedAt       time.Time `json:"added_at"`

	// 预警条件
	AlertOnUp     float64   `json:"alert_on_up"`      // 上涨超过N%预警
	AlertOnDown   float64   `json:"alert_on_down"`    // 下跌超过N%预警
	AlertOnPrice  float64   `json:"alert_on_price"`   // 价格到达预警
	AlertOnVolume float64   `json:"alert_on_volume"`  // 成交量放大N倍预警

	// 策略监控
	EnableStrategy bool     `json:"enable_strategy"`  // 是否启用策略分析

	// 最后状态
	LastPrice     float64   `json:"last_price"`
	LastUpdate    time.Time `json:"last_update"`
}

// MarketMonitor 市场监控器
type MarketMonitor struct {
	mu sync.RWMutex

	provider market.Provider
	watchList map[string]*WatchItem

	// 回调
	onAlert func(*Alert)

	// 运行状态
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	// 配置
	interval time.Duration // 刷新间隔
}

// NewMarketMonitor 创建市场监控器
func NewMarketMonitor(provider market.Provider) *MarketMonitor {
	if provider == nil {
		provider = market.NewSinaProvider()
	}

	return &MarketMonitor{
		provider:  provider,
		watchList: make(map[string]*WatchItem),
		interval:  5 * time.Second, // 默认5秒刷新
	}
}

// SetInterval 设置刷新间隔
func (m *MarketMonitor) SetInterval(interval time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.interval = interval
}

// OnAlert 注册预警回调
func (m *MarketMonitor) OnAlert(fn func(*Alert)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onAlert = fn
}

// AddWatch 添加监控股票
func (m *MarketMonitor) AddWatch(item *WatchItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if item.Stock == "" {
		return fmt.Errorf("stock code is required")
	}

	item.AddedAt = time.Now()
	m.watchList[item.Stock] = item

	slog.Info("monitor: watch added", "stock", item.Stock, "name", item.Name)
	return nil
}

// RemoveWatch 移除监控股票
func (m *MarketMonitor) RemoveWatch(stock string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.watchList, stock)
	slog.Info("monitor: watch removed", "stock", stock)
}

// GetWatchList 获取监控列表
func (m *MarketMonitor) GetWatchList() []*WatchItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]*WatchItem, 0, len(m.watchList))
	for _, item := range m.watchList {
		items = append(items, item)
	}
	return items
}

// Start 启动监控
func (m *MarketMonitor) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("monitor already running")
	}

	m.ctx, m.cancel = context.WithCancel(ctx)
	m.running = true
	m.mu.Unlock()

	slog.Info("monitor: started", "interval", m.interval)

	// 启动监控循环
	go m.monitorLoop()

	return nil
}

// Stop 停止监控
func (m *MarketMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	if m.cancel != nil {
		m.cancel()
	}
	m.running = false

	slog.Info("monitor: stopped")
}

// monitorLoop 监控循环
func (m *MarketMonitor) monitorLoop() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAll()
		}
	}
}

// checkAll 检查所有监控股票
func (m *MarketMonitor) checkAll() {
	m.mu.RLock()
	stocks := make([]string, 0, len(m.watchList))
	for stock := range m.watchList {
		stocks = append(stocks, stock)
	}
	m.mu.RUnlock()

	if len(stocks) == 0 {
		return
	}

	// 批量获取行情
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	quotes, err := m.provider.GetBatchQuotes(ctx, stocks)
	if err != nil {
		slog.Warn("monitor: get quotes failed", "err", err)
		return
	}

	// 检查每只股票
	for stock, quote := range quotes {
		m.checkStock(stock, quote)
	}
}

// checkStock 检查单只股票
func (m *MarketMonitor) checkStock(stock string, quote *trading.Quote) {
	m.mu.Lock()
	item, ok := m.watchList[stock]
	if !ok {
		m.mu.Unlock()
		return
	}

	lastPrice := item.LastPrice
	item.LastPrice = quote.Price
	item.LastUpdate = quote.Timestamp
	m.mu.Unlock()

	// 首次获取，不触发预警
	if lastPrice == 0 {
		return
	}

	// 检查预警条件
	m.checkAlerts(item, quote, lastPrice)
}

// checkAlerts 检查预警条件
func (m *MarketMonitor) checkAlerts(item *WatchItem, quote *trading.Quote, lastPrice float64) {
	// 1. 涨跌幅预警
	if item.AlertOnUp > 0 && quote.ChangePct >= item.AlertOnUp {
		m.emitAlert(&Alert{
			Type:      AlertPriceUp,
			Stock:     item.Stock,
			Message:   fmt.Sprintf("%s 上涨 %.2f%%", item.Name, quote.ChangePct*100),
			Price:     quote.Price,
			ChangePct: quote.ChangePct,
			Timestamp: quote.Timestamp,
		})
	}

	if item.AlertOnDown > 0 && quote.ChangePct <= -item.AlertOnDown {
		m.emitAlert(&Alert{
			Type:      AlertPriceDown,
			Stock:     item.Stock,
			Message:   fmt.Sprintf("%s 下跌 %.2f%%", item.Name, quote.ChangePct*100),
			Price:     quote.Price,
			ChangePct: quote.ChangePct,
			Timestamp: quote.Timestamp,
		})
	}

	// 2. 价格预警
	if item.AlertOnPrice > 0 {
		if lastPrice < item.AlertOnPrice && quote.Price >= item.AlertOnPrice {
			m.emitAlert(&Alert{
				Type:      AlertBreakHigh,
				Stock:     item.Stock,
				Message:   fmt.Sprintf("%s 突破 %.2f", item.Name, item.AlertOnPrice),
				Price:     quote.Price,
				ChangePct: quote.ChangePct,
				Timestamp: quote.Timestamp,
			})
		} else if lastPrice > item.AlertOnPrice && quote.Price <= item.AlertOnPrice {
			m.emitAlert(&Alert{
				Type:      AlertBreakLow,
				Stock:     item.Stock,
				Message:   fmt.Sprintf("%s 跌破 %.2f", item.Name, item.AlertOnPrice),
				Price:     quote.Price,
				ChangePct: quote.ChangePct,
				Timestamp: quote.Timestamp,
			})
		}
	}

	// 3. 涨跌停预警
	if quote.ChangePct >= 0.099 {
		m.emitAlert(&Alert{
			Type:      AlertLimitUp,
			Stock:     item.Stock,
			Message:   fmt.Sprintf("%s 涨停", item.Name),
			Price:     quote.Price,
			ChangePct: quote.ChangePct,
			Timestamp: quote.Timestamp,
		})
	} else if quote.ChangePct <= -0.099 {
		m.emitAlert(&Alert{
			Type:      AlertLimitDown,
			Stock:     item.Stock,
			Message:   fmt.Sprintf("%s 跌停", item.Name),
			Price:     quote.Price,
			ChangePct: quote.ChangePct,
			Timestamp: quote.Timestamp,
		})
	}
}

// emitAlert 发送预警
func (m *MarketMonitor) emitAlert(alert *Alert) {
	slog.Info("monitor: alert",
		"type", alert.Type,
		"stock", alert.Stock,
		"message", alert.Message,
		"price", alert.Price)

	m.mu.RLock()
	callback := m.onAlert
	m.mu.RUnlock()

	if callback != nil {
		callback(alert)
	}
}

// GetSnapshot 获取当前快照
func (m *MarketMonitor) GetSnapshot() map[string]*trading.Quote {
	m.mu.RLock()
	stocks := make([]string, 0, len(m.watchList))
	for stock := range m.watchList {
		stocks = append(stocks, stock)
	}
	m.mu.RUnlock()

	if len(stocks) == 0 {
		return make(map[string]*trading.Quote)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	quotes, err := m.provider.GetBatchQuotes(ctx, stocks)
	if err != nil {
		slog.Warn("monitor: get snapshot failed", "err", err)
		return make(map[string]*trading.Quote)
	}

	return quotes
}
