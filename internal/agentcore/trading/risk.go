package trading

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// RiskEngine — 风控引擎
// 三层风控：Pre-trade / In-trade / Post-trade
// ──────────────────────────────────────────────

// RiskEngine 风控引擎
type RiskEngine struct {
	mu sync.RWMutex

	broker   Broker
	config   *TradingConfig
	portfolio *PortfolioManager

	// 熔断器
	circuitBreaker *CircuitBreaker

	// 统计数据
	dailyTrades int
	dailyLoss   float64
	lastReset   time.Time
}

// NewRiskEngine 创建风控引擎
func NewRiskEngine(broker Broker, config *TradingConfig, portfolio *PortfolioManager) *RiskEngine {
	return &RiskEngine{
		broker:    broker,
		config:    config,
		portfolio: portfolio,
		circuitBreaker: &CircuitBreaker{
			MaxDailyLoss:    config.MaxDailyLoss,
			MaxDrawdown:     config.MaxDrawdown,
			VolatilitySpike: config.VolatilityLimit,
			ConsecutiveLoss: 3,
		},
		lastReset: time.Now(),
	}
}

// CheckOrder 检查订单（Pre-trade 风控）
func (re *RiskEngine) CheckOrder(ctx context.Context, order *Order) *RiskCheck {
	re.mu.Lock()
	defer re.mu.Unlock()

	// 重置每日统计
	re.resetDailyStatsIfNeeded()

	result := &RiskCheck{
		Passed: true,
		Alerts: make([]string, 0),
	}

	// 1. 熔断器检查
	if re.circuitBreaker.IsTripped() {
		result.Passed = false
		result.Reason = fmt.Sprintf("circuit breaker tripped: %s", re.circuitBreaker.TripReason())
		return result
	}

	// 2. 每日交易次数限制
	if re.dailyTrades >= re.config.MaxDailyTrades {
		result.Passed = false
		result.Reason = fmt.Sprintf("daily trade limit reached: %d/%d", re.dailyTrades, re.config.MaxDailyTrades)
		return result
	}

	// 3. 每日亏损限制
	if re.dailyLoss >= re.config.MaxDailyLoss {
		result.Passed = false
		result.Reason = fmt.Sprintf("daily loss limit reached: %.2f%%", re.dailyLoss*100)
		return result
	}

	// 4. 仓位检查
	if order.Action == OrderBuy {
		// 单只股票仓位限制
		currentRatio := re.portfolio.GetPositionRatio(order.Stock)
		quote, err := re.broker.GetQuote(ctx, order.Stock)
		if err != nil {
			result.Passed = false
			result.Reason = fmt.Sprintf("get quote failed: %v", err)
			return result
		}

		portfolio := re.portfolio.GetPortfolio()
		if portfolio == nil {
			result.Passed = false
			result.Reason = "portfolio not available"
			return result
		}

		orderValue := float64(order.Quantity) * quote.Price
		newRatio := (currentRatio*portfolio.TotalValue + orderValue) / portfolio.TotalValue

		if newRatio > re.config.MaxPositionSingle {
			result.Passed = false
			result.Reason = fmt.Sprintf("position limit exceeded: %.2f%% > %.2f%%",
				newRatio*100, re.config.MaxPositionSingle*100)
			return result
		}

		// 总仓位限制
		totalRatio := re.portfolio.GetTotalPositionRatio()
		newTotalRatio := (totalRatio*portfolio.TotalValue + orderValue) / portfolio.TotalValue

		if newTotalRatio > re.config.MaxPositionTotal {
			result.Passed = false
			result.Reason = fmt.Sprintf("total position limit exceeded: %.2f%% > %.2f%%",
				newTotalRatio*100, re.config.MaxPositionTotal*100)
			return result
		}

		// 资金检查
		if orderValue > portfolio.Cash {
			result.Passed = false
			result.Reason = fmt.Sprintf("insufficient cash: need %.2f, have %.2f",
				orderValue, portfolio.Cash)
			return result
		}
	}

	// 5. 涨跌停检查
	quote, err := re.broker.GetQuote(ctx, order.Stock)
	if err == nil {
		if quote.ChangePct >= 0.099 {
			result.Alerts = append(result.Alerts, "stock near limit up")
		}
		if quote.ChangePct <= -0.099 {
			result.Alerts = append(result.Alerts, "stock near limit down")
		}
	}

	// 6. 波动率检查
	if quote != nil && abs(quote.ChangePct) > re.config.VolatilityLimit {
		result.Passed = false
		result.Reason = fmt.Sprintf("volatility too high: %.2f%% > %.2f%%",
			abs(quote.ChangePct)*100, re.config.VolatilityLimit*100)
		return result
	}

	return result
}

// RecordTrade 记录交易（用于统计）
func (re *RiskEngine) RecordTrade(order *Order, pnl float64) {
	re.mu.Lock()
	defer re.mu.Unlock()

	re.dailyTrades++

	if pnl < 0 {
		re.dailyLoss += abs(pnl)
		re.circuitBreaker.RecordLoss()
	} else {
		re.circuitBreaker.ResetConsecutiveLoss()
	}

	// 检查是否触发熔断
	portfolio := re.portfolio.GetPortfolio()
	if portfolio != nil {
		dailyLossRatio := re.dailyLoss / portfolio.TotalValue
		if dailyLossRatio >= re.config.MaxDailyLoss {
			re.circuitBreaker.Trip("daily loss limit exceeded")
			slog.Warn("trading: circuit breaker tripped", "reason", "daily loss limit")
		}
	}
}

// CheckDrawdown 检查回撤
func (re *RiskEngine) CheckDrawdown(ctx context.Context) error {
	portfolio := re.portfolio.GetPortfolio()
	if portfolio == nil {
		return nil
	}

	// 计算回撤（简化版，实际需要记录历史最高净值）
	// 这里假设初始资金为 TotalValue
	// 实际应该维护一个 highWaterMark

	// TODO: 实现完整的回撤计算
	return nil
}

// GetCircuitBreaker 获取熔断器
func (re *RiskEngine) GetCircuitBreaker() *CircuitBreaker {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return re.circuitBreaker
}

// ResetCircuitBreaker 重置熔断器
func (re *RiskEngine) ResetCircuitBreaker() {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.circuitBreaker.Reset()
	slog.Info("trading: circuit breaker reset")
}

// GetDailyStats 获取每日统计
func (re *RiskEngine) GetDailyStats() (trades int, loss float64) {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return re.dailyTrades, re.dailyLoss
}

// resetDailyStatsIfNeeded 如果需要则重置每日统计
func (re *RiskEngine) resetDailyStatsIfNeeded() {
	now := time.Now()
	if now.Day() != re.lastReset.Day() {
		re.dailyTrades = 0
		re.dailyLoss = 0
		re.lastReset = now
		slog.Info("trading: daily stats reset")
	}
}

// ──────────────────────────────────────────────
// CircuitBreaker — 熔断器
// ──────────────────────────────────────────────

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	mu sync.RWMutex

	// 阈值
	MaxDailyLoss    float64 // 当日最大亏损比例
	MaxDrawdown     float64 // 最大回撤比例
	VolatilitySpike float64 // 波动率飙升阈值
	ConsecutiveLoss int     // 连续亏损次数

	// 状态
	tripped         bool
	tripReason      string
	trippedAt       time.Time
	consecutiveLoss int
}

// IsTripped 是否已触发
func (cb *CircuitBreaker) IsTripped() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.tripped
}

// Trip 触发熔断
func (cb *CircuitBreaker) Trip(reason string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.tripped {
		return
	}

	cb.tripped = true
	cb.tripReason = reason
	cb.trippedAt = time.Now()

	slog.Warn("trading: circuit breaker tripped", "reason", reason)
}

// Reset 重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.tripped = false
	cb.tripReason = ""
	cb.consecutiveLoss = 0
}

// TripReason 获取触发原因
func (cb *CircuitBreaker) TripReason() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.tripReason
}

// RecordLoss 记录亏损
func (cb *CircuitBreaker) RecordLoss() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveLoss++

	if cb.consecutiveLoss >= cb.ConsecutiveLoss {
		cb.tripped = true
		cb.tripReason = fmt.Sprintf("consecutive loss limit: %d", cb.consecutiveLoss)
		cb.trippedAt = time.Now()
		slog.Warn("trading: circuit breaker tripped", "reason", cb.tripReason)
	}
}

// ResetConsecutiveLoss 重置连续亏损计数
func (cb *CircuitBreaker) ResetConsecutiveLoss() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutiveLoss = 0
}

// abs 绝对值
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
