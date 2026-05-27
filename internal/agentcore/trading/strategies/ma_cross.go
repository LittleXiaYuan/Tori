package strategies

import (
	"context"
	"fmt"

	"yunque-agent/internal/agentcore/trading"
)

// ──────────────────────────────────────────────
// MACrossStrategy — 均线交叉策略
// 短期均线上穿长期均线 → 买入信号（金叉）
// 短期均线下穿长期均线 → 卖出信号（死叉）
// ──────────────────────────────────────────────

// MACrossStrategy 均线交叉策略
type MACrossStrategy struct {
	shortPeriod int // 短期均线周期（默认5）
	longPeriod  int // 长期均线周期（默认20）
}

// NewMACrossStrategy 创建均线交叉策略
func NewMACrossStrategy(shortPeriod, longPeriod int) *MACrossStrategy {
	if shortPeriod <= 0 {
		shortPeriod = 5
	}
	if longPeriod <= 0 {
		longPeriod = 20
	}

	return &MACrossStrategy{
		shortPeriod: shortPeriod,
		longPeriod:  longPeriod,
	}
}

// Name 策略名称
func (s *MACrossStrategy) Name() string {
	return "ma_cross"
}

// Analyze 分析行情，生成信号
func (s *MACrossStrategy) Analyze(ctx context.Context, stock string, klines []trading.KLine) (*trading.Signal, error) {
	if len(klines) < s.longPeriod+1 {
		return nil, fmt.Errorf("insufficient data: need %d, got %d", s.longPeriod+1, len(klines))
	}

	// 计算当前和前一根K线的短期、长期均线
	currentShortMA := s.calculateMA(klines, s.shortPeriod, 0)
	currentLongMA := s.calculateMA(klines, s.longPeriod, 0)

	prevShortMA := s.calculateMA(klines, s.shortPeriod, 1)
	prevLongMA := s.calculateMA(klines, s.longPeriod, 1)

	// 调试输出
	// fmt.Printf("MA Analysis: prevShort=%.2f prevLong=%.2f currentShort=%.2f currentLong=%.2f\n",
	// 	prevShortMA, prevLongMA, currentShortMA, currentLongMA)

	// 检测金叉（买入信号）
	if prevShortMA <= prevLongMA && currentShortMA > currentLongMA {
		// 金叉：短期均线上穿长期均线
		confidence := s.calculateConfidence(currentShortMA, currentLongMA, klines)

		return &trading.Signal{
			Type:       trading.SignalBuy,
			Stock:      stock,
			Price:      klines[len(klines)-1].Close,
			Confidence: confidence,
			Reason:     fmt.Sprintf("MA金叉: MA%d(%.2f) 上穿 MA%d(%.2f)", s.shortPeriod, currentShortMA, s.longPeriod, currentLongMA),
			Strategy:   s.Name(),
			Metadata: map[string]any{
				"short_ma": currentShortMA,
				"long_ma":  currentLongMA,
			},
		}, nil
	}

	// 检测死叉（卖出信号）
	if prevShortMA >= prevLongMA && currentShortMA < currentLongMA {
		// 死叉：短期均线下穿长期均线
		confidence := s.calculateConfidence(currentLongMA, currentShortMA, klines)

		return &trading.Signal{
			Type:       trading.SignalSell,
			Stock:      stock,
			Price:      klines[len(klines)-1].Close,
			Confidence: confidence,
			Reason:     fmt.Sprintf("MA死叉: MA%d(%.2f) 下穿 MA%d(%.2f)", s.shortPeriod, currentShortMA, s.longPeriod, currentLongMA),
			Strategy:   s.Name(),
			Metadata: map[string]any{
				"short_ma": currentShortMA,
				"long_ma":  currentLongMA,
			},
		}, nil
	}

	// 无信号 - 返回 nil 而不是 Hold 信号
	return nil, nil
}

// Config 策略配置
func (s *MACrossStrategy) Config() map[string]any {
	return map[string]any{
		"short_period": s.shortPeriod,
		"long_period":  s.longPeriod,
	}
}

// calculateMA 计算移动平均线
// offset: 0=最新, 1=前一根, 2=前两根...
func (s *MACrossStrategy) calculateMA(klines []trading.KLine, period int, offset int) float64 {
	if len(klines) < period+offset {
		return 0
	}

	sum := 0.0
	start := len(klines) - period - offset
	end := len(klines) - offset

	for i := start; i < end; i++ {
		sum += klines[i].Close
	}

	return sum / float64(period)
}

// calculateConfidence 计算信号置信度
func (s *MACrossStrategy) calculateConfidence(ma1, ma2 float64, klines []trading.KLine) float64 {
	// 基础置信度
	baseConfidence := 0.6

	// 1. 均线距离：距离越大，置信度越高
	distance := abs(ma1 - ma2)
	distanceRatio := distance / ma2
	distanceScore := min(distanceRatio*10, 0.2) // 最多加0.2

	// 2. 成交量：最近成交量放大，置信度提高
	volumeScore := 0.0
	if len(klines) >= 10 {
		recentVolume := float64(klines[len(klines)-1].Volume)
		avgVolume := s.calculateAvgVolume(klines, 10)
		if recentVolume > avgVolume*1.5 {
			volumeScore = 0.1
		}
	}

	// 3. 趋势强度：价格连续上涨/下跌，置信度提高
	trendScore := s.calculateTrendScore(klines, 5)

	confidence := baseConfidence + distanceScore + volumeScore + trendScore

	// 限制在 [0, 1] 范围
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

// calculateAvgVolume 计算平均成交量
func (s *MACrossStrategy) calculateAvgVolume(klines []trading.KLine, period int) float64 {
	if len(klines) < period {
		return 0
	}

	sum := int64(0)
	start := len(klines) - period

	for i := start; i < len(klines); i++ {
		sum += klines[i].Volume
	}

	return float64(sum) / float64(period)
}

// calculateTrendScore 计算趋势得分
func (s *MACrossStrategy) calculateTrendScore(klines []trading.KLine, period int) float64 {
	if len(klines) < period {
		return 0
	}

	upCount := 0
	downCount := 0
	start := len(klines) - period

	for i := start + 1; i < len(klines); i++ {
		if klines[i].Close > klines[i-1].Close {
			upCount++
		} else if klines[i].Close < klines[i-1].Close {
			downCount++
		}
	}

	// 连续上涨或下跌的比例
	ratio := float64(max(upCount, downCount)) / float64(period-1)

	// 最多加0.1
	return min(ratio*0.1, 0.1)
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
