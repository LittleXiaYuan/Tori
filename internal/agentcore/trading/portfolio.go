package trading

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// PortfolioManager — 投资组合管理器
// 负责仓位计算、资金分配、信号汇总决策
// ──────────────────────────────────────────────

// PortfolioManager 投资组合管理器
type PortfolioManager struct {
	mu sync.RWMutex

	broker Broker
	config *TradingConfig

	// 当前组合
	portfolio *Portfolio
	lastUpdate time.Time
}

// NewPortfolioManager 创建投资组合管理器
func NewPortfolioManager(broker Broker, config *TradingConfig) *PortfolioManager {
	return &PortfolioManager{
		broker: broker,
		config: config,
	}
}

// RefreshPortfolio 刷新投资组合
func (pm *PortfolioManager) RefreshPortfolio(ctx context.Context) error {
	portfolio, err := pm.broker.GetPortfolio(ctx)
	if err != nil {
		return fmt.Errorf("get portfolio: %w", err)
	}

	pm.mu.Lock()
	pm.portfolio = portfolio
	pm.lastUpdate = time.Now()
	pm.mu.Unlock()

	return nil
}

// GetPortfolio 获取投资组合
func (pm *PortfolioManager) GetPortfolio() *Portfolio {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.portfolio
}

// DecideOrders 根据信号决策订单（多策略投票制）
func (pm *PortfolioManager) DecideOrders(ctx context.Context, signals []*Signal) ([]*Order, error) {
	if len(signals) == 0 {
		return nil, nil
	}

	// 刷新组合
	if err := pm.RefreshPortfolio(ctx); err != nil {
		return nil, err
	}

	pm.mu.RLock()
	portfolio := pm.portfolio
	config := pm.config
	pm.mu.RUnlock()

	// 按股票分组信号
	stockSignals := make(map[string][]*Signal)
	for _, sig := range signals {
		stockSignals[sig.Stock] = append(stockSignals[sig.Stock], sig)
	}

	orders := make([]*Order, 0)

	// 对每只股票进行决策
	for stock, sigs := range stockSignals {
		// 1. 检查黑名单
		if pm.isBlacklisted(stock) {
			slog.Debug("trading: stock in blacklist", "stock", stock)
			continue
		}

		// 2. 加权汇总信号
		buyScore, sellScore := pm.aggregateSignals(sigs)

		// 3. 决策
		var action OrderAction
		var confidence float64

		if buyScore > sellScore && buyScore > config.SignalThreshold {
			action = OrderBuy
			confidence = buyScore
		} else if sellScore > buyScore && sellScore > config.SignalThreshold {
			action = OrderSell
			confidence = sellScore
		} else {
			// 信号不明确，不操作
			continue
		}

		// 4. 计算数量
		quantity, err := pm.calculateQuantity(stock, action, confidence, portfolio)
		if err != nil {
			slog.Warn("trading: calculate quantity failed", "stock", stock, "err", err)
			continue
		}

		if quantity <= 0 {
			continue
		}

		// 5. 获取当前价格
		quote, err := pm.broker.GetQuote(ctx, stock)
		if err != nil {
			slog.Warn("trading: get quote failed", "stock", stock, "err", err)
			continue
		}

		// 6. 创建订单
		order := &Order{
			ID:        fmt.Sprintf("order_%s_%d", stock, time.Now().Unix()),
			Action:    action,
			Stock:     stock,
			Quantity:  quantity,
			Price:     quote.Price,
			Strategy:  pm.getMainStrategy(sigs),
			Timestamp: time.Now(),
			Status:    OrderPending,
			Metadata: map[string]any{
				"confidence": confidence,
				"signals":    len(sigs),
			},
		}

		orders = append(orders, order)
	}

	return orders, nil
}

// aggregateSignals 加权汇总信号
func (pm *PortfolioManager) aggregateSignals(signals []*Signal) (buyScore, sellScore float64) {
	pm.mu.RLock()
	weights := pm.config.StrategyWeights
	pm.mu.RUnlock()

	for _, sig := range signals {
		weight := weights[sig.Strategy]
		if weight == 0 {
			weight = 1.0 // 默认权重
		}

		score := sig.Confidence * weight

		switch sig.Type {
		case SignalBuy:
			buyScore += score
		case SignalSell:
			sellScore += score
		}
	}

	// 归一化
	total := buyScore + sellScore
	if total > 0 {
		buyScore /= total
		sellScore /= total
	}

	return buyScore, sellScore
}

// calculateQuantity 计算交易数量
func (pm *PortfolioManager) calculateQuantity(stock string, action OrderAction, confidence float64, portfolio *Portfolio) (int, error) {
	pm.mu.RLock()
	config := pm.config
	pm.mu.RUnlock()

	if action == OrderBuy {
		// 买入：根据可用资金和仓位限制计算
		maxValue := portfolio.TotalValue * config.MaxPositionSingle
		availableCash := portfolio.Cash

		// 考虑置信度调整仓位
		targetValue := maxValue * confidence
		if targetValue > availableCash {
			targetValue = availableCash
		}

		// 获取当前价格
		quote, err := pm.broker.GetQuote(context.Background(), stock)
		if err != nil {
			return 0, err
		}

		quantity := int(targetValue / quote.Price)
		// 确保是100的整数倍（A股交易规则）
		quantity = (quantity / 100) * 100

		return quantity, nil
	}

	// 卖出：卖出当前持仓
	if pos, ok := portfolio.Positions[stock]; ok {
		// 根据置信度决定卖出比例
		sellRatio := confidence
		quantity := int(float64(pos.Quantity) * sellRatio)
		quantity = (quantity / 100) * 100
		return quantity, nil
	}

	return 0, nil
}

// getMainStrategy 获取主要策略名称
func (pm *PortfolioManager) getMainStrategy(signals []*Signal) string {
	if len(signals) == 0 {
		return ""
	}

	// 返回置信度最高的策略
	maxConf := 0.0
	mainStrategy := signals[0].Strategy

	for _, sig := range signals {
		if sig.Confidence > maxConf {
			maxConf = sig.Confidence
			mainStrategy = sig.Strategy
		}
	}

	return mainStrategy
}

// isBlacklisted 检查是否在黑名单
func (pm *PortfolioManager) isBlacklisted(stock string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, b := range pm.config.Blacklist {
		if b == stock {
			return true
		}
	}
	return false
}

// GetPositionRatio 获取持仓比例
func (pm *PortfolioManager) GetPositionRatio(stock string) float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.portfolio == nil {
		return 0
	}

	pos, ok := pm.portfolio.Positions[stock]
	if !ok {
		return 0
	}

	if pm.portfolio.TotalValue == 0 {
		return 0
	}

	return pos.MarketValue / pm.portfolio.TotalValue
}

// GetTotalPositionRatio 获取总仓位比例
func (pm *PortfolioManager) GetTotalPositionRatio() float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.portfolio == nil || pm.portfolio.TotalValue == 0 {
		return 0
	}

	positionValue := pm.portfolio.TotalValue - pm.portfolio.Cash
	return positionValue / pm.portfolio.TotalValue
}
