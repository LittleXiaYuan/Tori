package monitor

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/trading"
	"yunque-agent/packs/trading/market"
)

// ──────────────────────────────────────────────
// Dashboard — 看盘面板
// 提供实时市场概览、自选股、涨跌排行
// ──────────────────────────────────────────────

// Dashboard 看盘面板
type Dashboard struct {
	provider market.Provider
	monitor  *MarketMonitor
}

// NewDashboard 创建看盘面板
func NewDashboard(provider market.Provider, monitor *MarketMonitor) *Dashboard {
	if provider == nil {
		provider = market.NewSinaProvider()
	}

	return &Dashboard{
		provider: provider,
		monitor:  monitor,
	}
}

// MarketOverview 市场概览
type MarketOverview struct {
	Timestamp    time.Time          `json:"timestamp"`
	WatchList    []*StockSnapshot   `json:"watch_list"`
	TopGainers   []*StockSnapshot   `json:"top_gainers"`
	TopLosers    []*StockSnapshot   `json:"top_losers"`
	Alerts       []*Alert           `json:"alerts"`
	Summary      *MarketSummary     `json:"summary"`
}

// StockSnapshot 股票快照
type StockSnapshot struct {
	Stock     string  `json:"stock"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Change    float64 `json:"change"`
	ChangePct float64 `json:"change_pct"`
	Volume    int64   `json:"volume"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Open      float64 `json:"open"`
}

// MarketSummary 市场摘要
type MarketSummary struct {
	TotalStocks   int     `json:"total_stocks"`
	UpCount       int     `json:"up_count"`
	DownCount     int     `json:"down_count"`
	FlatCount     int     `json:"flat_count"`
	AvgChangePct  float64 `json:"avg_change_pct"`
	LimitUpCount  int     `json:"limit_up_count"`
	LimitDownCount int    `json:"limit_down_count"`
}

// GetOverview 获取市场概览
func (d *Dashboard) GetOverview(ctx context.Context) (*MarketOverview, error) {
	overview := &MarketOverview{
		Timestamp: time.Now(),
		Alerts:    make([]*Alert, 0),
	}

	// 1. 获取自选股行情
	if d.monitor != nil {
		watchList := d.monitor.GetWatchList()
		if len(watchList) > 0 {
			stocks := make([]string, len(watchList))
			stockNames := make(map[string]string)
			for i, item := range watchList {
				stocks[i] = item.Stock
				stockNames[item.Stock] = item.Name
			}

			quotes, err := d.provider.GetBatchQuotes(ctx, stocks)
			if err == nil {
				snapshots := make([]*StockSnapshot, 0, len(quotes))
				for stock, quote := range quotes {
					snapshots = append(snapshots, &StockSnapshot{
						Stock:     stock,
						Name:      stockNames[stock],
						Price:     quote.Price,
						Change:    quote.Change,
						ChangePct: quote.ChangePct,
						Volume:    quote.Volume,
						High:      quote.High,
						Low:       quote.Low,
						Open:      quote.Open,
					})
				}

				// 按涨跌幅排序
				sort.Slice(snapshots, func(i, j int) bool {
					return snapshots[i].ChangePct > snapshots[j].ChangePct
				})

				overview.WatchList = snapshots
			}
		}
	}

	// 2. 计算市场摘要
	if len(overview.WatchList) > 0 {
		summary := &MarketSummary{
			TotalStocks: len(overview.WatchList),
		}

		totalChangePct := 0.0
		for _, snap := range overview.WatchList {
			if snap.ChangePct > 0 {
				summary.UpCount++
			} else if snap.ChangePct < 0 {
				summary.DownCount++
			} else {
				summary.FlatCount++
			}

			if snap.ChangePct >= 0.099 {
				summary.LimitUpCount++
			} else if snap.ChangePct <= -0.099 {
				summary.LimitDownCount++
			}

			totalChangePct += snap.ChangePct
		}

		summary.AvgChangePct = totalChangePct / float64(summary.TotalStocks)
		overview.Summary = summary

		// 3. 提取涨跌幅前3
		if len(overview.WatchList) > 0 {
			// 涨幅榜
			count := min(3, len(overview.WatchList))
			overview.TopGainers = overview.WatchList[:count]

			// 跌幅榜
			overview.TopLosers = make([]*StockSnapshot, 0, count)
			for i := len(overview.WatchList) - 1; i >= 0 && len(overview.TopLosers) < count; i-- {
				if overview.WatchList[i].ChangePct < 0 {
					overview.TopLosers = append(overview.TopLosers, overview.WatchList[i])
				}
			}
		}
	}

	return overview, nil
}

// FormatOverview 格式化市场概览（文本输出）
func (d *Dashboard) FormatOverview(overview *MarketOverview) string {
	var sb strings.Builder

	sb.WriteString("📊 市场概览\n")
	sb.WriteString(fmt.Sprintf("更新时间: %s\n\n", overview.Timestamp.Format("15:04:05")))

	// 市场摘要
	if overview.Summary != nil {
		s := overview.Summary
		sb.WriteString("📈 市场摘要\n")
		sb.WriteString(fmt.Sprintf("总计: %d只 | 上涨: %d | 下跌: %d | 平盘: %d\n",
			s.TotalStocks, s.UpCount, s.DownCount, s.FlatCount))
		sb.WriteString(fmt.Sprintf("平均涨跌: %.2f%% | 涨停: %d | 跌停: %d\n\n",
			s.AvgChangePct*100, s.LimitUpCount, s.LimitDownCount))
	}

	// 涨幅榜
	if len(overview.TopGainers) > 0 {
		sb.WriteString("🔥 涨幅榜\n")
		for i, snap := range overview.TopGainers {
			sb.WriteString(fmt.Sprintf("%d. %s (%s) %.2f (%.2f%%)\n",
				i+1, snap.Name, snap.Stock, snap.Price, snap.ChangePct*100))
		}
		sb.WriteString("\n")
	}

	// 跌幅榜
	if len(overview.TopLosers) > 0 {
		sb.WriteString("📉 跌幅榜\n")
		for i, snap := range overview.TopLosers {
			sb.WriteString(fmt.Sprintf("%d. %s (%s) %.2f (%.2f%%)\n",
				i+1, snap.Name, snap.Stock, snap.Price, snap.ChangePct*100))
		}
		sb.WriteString("\n")
	}

	// 自选股
	if len(overview.WatchList) > 0 {
		sb.WriteString("⭐ 自选股\n")
		for _, snap := range overview.WatchList {
			emoji := "📊"
			if snap.ChangePct > 0 {
				emoji = "📈"
			} else if snap.ChangePct < 0 {
				emoji = "📉"
			}

			sb.WriteString(fmt.Sprintf("%s %s (%s)\n", emoji, snap.Name, snap.Stock))
			sb.WriteString(fmt.Sprintf("   价格: %.2f | 涨跌: %.2f (%.2f%%)\n",
				snap.Price, snap.Change, snap.ChangePct*100))
			sb.WriteString(fmt.Sprintf("   最高: %.2f | 最低: %.2f | 成交量: %d\n",
				snap.High, snap.Low, snap.Volume))
		}
	}

	return sb.String()
}

// GetStockDetail 获取股票详情
func (d *Dashboard) GetStockDetail(ctx context.Context, stock string) (map[string]any, error) {
	// 获取实时行情
	quote, err := d.provider.GetQuote(ctx, stock)
	if err != nil {
		return nil, fmt.Errorf("get quote: %w", err)
	}

	// 获取K线数据
	klines, err := d.provider.GetKLines(ctx, stock, "1d", 30)
	if err != nil {
		return nil, fmt.Errorf("get klines: %w", err)
	}

	// 计算技术指标
	ma5 := calculateMA(klines, 5)
	ma10 := calculateMA(klines, 10)
	ma20 := calculateMA(klines, 20)

	result := map[string]any{
		"stock": stock,
		"quote": quote,
		"klines": klines,
		"indicators": map[string]any{
			"ma5":  ma5,
			"ma10": ma10,
			"ma20": ma20,
		},
	}

	return result, nil
}

// calculateMA 计算移动平均线
func calculateMA(klines []trading.KLine, period int) float64 {
	if len(klines) < period {
		return 0
	}

	sum := 0.0
	start := len(klines) - period

	for i := start; i < len(klines); i++ {
		sum += klines[i].Close
	}

	return sum / float64(period)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
