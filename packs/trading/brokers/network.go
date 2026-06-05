package brokers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/trading"
	"yunque-agent/packs/trading/market"
)

// ──────────────────────────────────────────────
// NetworkBroker — 网络券商基类
// 使用真实行情数据 + 模拟账户
// 适合测试和小资金实盘
// ──────────────────────────────────────────────

// NetworkBroker 网络券商（真实行情 + 模拟账户）
type NetworkBroker struct {
	mu sync.RWMutex

	name           string
	marketProvider market.Provider

	// 模拟账户
	cash       float64
	positions  map[string]*trading.Position
	totalValue float64

	// 订单记录
	orders map[string]*trading.Order
}

// NewNetworkBroker 创建网络券商
func NewNetworkBroker(name string, initialCash float64, provider market.Provider) *NetworkBroker {
	if provider == nil {
		provider = market.NewSinaProvider()
	}

	return &NetworkBroker{
		name:           name,
		marketProvider: provider,
		cash:           initialCash,
		positions:      make(map[string]*trading.Position),
		totalValue:     initialCash,
		orders:         make(map[string]*trading.Order),
	}
}

// Name 券商名称
func (nb *NetworkBroker) Name() string {
	return nb.name
}

// GetQuote 获取实时行情（真实数据）
func (nb *NetworkBroker) GetQuote(ctx context.Context, stock string) (*trading.Quote, error) {
	return nb.marketProvider.GetQuote(ctx, stock)
}

// GetKLines 获取K线数据（真实数据）
func (nb *NetworkBroker) GetKLines(ctx context.Context, stock string, period string, count int) ([]trading.KLine, error) {
	return nb.marketProvider.GetKLines(ctx, stock, period, count)
}

// GetPortfolio 获取投资组合
func (nb *NetworkBroker) GetPortfolio(ctx context.Context) (*trading.Portfolio, error) {
	nb.mu.RLock()
	defer nb.mu.RUnlock()

	// 更新持仓市值（使用真实行情）
	totalValue := nb.cash
	positions := make(map[string]*trading.Position)

	for stock, pos := range nb.positions {
		quote, err := nb.marketProvider.GetQuote(ctx, stock)
		if err != nil {
			// 如果获取行情失败，使用上次的价格
			positions[stock] = pos
			totalValue += pos.MarketValue
			continue
		}

		currentPrice := quote.Price
		marketValue := float64(pos.Quantity) * currentPrice
		pnl := marketValue - float64(pos.Quantity)*pos.AvgCost
		pnlPercent := 0.0
		if pos.AvgCost > 0 {
			pnlPercent = pnl / (float64(pos.Quantity) * pos.AvgCost)
		}

		newPos := &trading.Position{
			Stock:        stock,
			Quantity:     pos.Quantity,
			AvgCost:      pos.AvgCost,
			CurrentPrice: currentPrice,
			MarketValue:  marketValue,
			PnL:          pnl,
			PnLPercent:   pnlPercent,
			UpdatedAt:    quote.Timestamp,
		}

		positions[stock] = newPos
		totalValue += marketValue
	}

	return &trading.Portfolio{
		Cash:       nb.cash,
		TotalValue: totalValue,
		Positions:  positions,
		UpdatedAt:  time.Now(),
	}, nil
}

// SubmitOrder 提交订单（模拟成交）
func (nb *NetworkBroker) SubmitOrder(ctx context.Context, order *trading.Order) error {
	// 先获取实时价格
	quote, err := nb.marketProvider.GetQuote(ctx, order.Stock)
	if err != nil {
		return fmt.Errorf("get quote: %w", err)
	}

	price := quote.Price
	totalCost := float64(order.Quantity) * price

	// 再获取锁进行账户操作
	nb.mu.Lock()
	defer nb.mu.Unlock()

	switch order.Action {
	case trading.OrderBuy:
		// 检查资金
		if totalCost > nb.cash {
			order.Status = trading.OrderRejected
			return fmt.Errorf("insufficient cash: need %.2f, have %.2f", totalCost, nb.cash)
		}

		// 扣除资金
		nb.cash -= totalCost

		// 更新持仓
		if pos, ok := nb.positions[order.Stock]; ok {
			// 已有持仓，计算新的平均成本
			totalQty := pos.Quantity + order.Quantity
			totalCostBasis := float64(pos.Quantity)*pos.AvgCost + totalCost
			pos.Quantity = totalQty
			pos.AvgCost = totalCostBasis / float64(totalQty)
		} else {
			// 新建持仓
			nb.positions[order.Stock] = &trading.Position{
				Stock:    order.Stock,
				Quantity: order.Quantity,
				AvgCost:  price,
			}
		}

	case trading.OrderSell:
		// 检查持仓
		pos, ok := nb.positions[order.Stock]
		if !ok || pos.Quantity < order.Quantity {
			order.Status = trading.OrderRejected
			return fmt.Errorf("insufficient position")
		}

		// 增加资金
		nb.cash += totalCost

		// 更新持仓
		pos.Quantity -= order.Quantity
		if pos.Quantity == 0 {
			delete(nb.positions, order.Stock)
		}
	}

	// 更新订单状态
	order.Status = trading.OrderFilled
	order.FilledQty = order.Quantity
	order.AvgPrice = price
	nb.orders[order.ID] = order

	return nil
}

// CancelOrder 撤销订单
func (nb *NetworkBroker) CancelOrder(ctx context.Context, orderID string) error {
	nb.mu.Lock()
	defer nb.mu.Unlock()

	order, ok := nb.orders[orderID]
	if !ok {
		return fmt.Errorf("order not found: %s", orderID)
	}

	if order.Status == trading.OrderFilled {
		return fmt.Errorf("order already filled")
	}

	order.Status = trading.OrderCancelled
	return nil
}

// GetOrder 查询订单状态
func (nb *NetworkBroker) GetOrder(ctx context.Context, orderID string) (*trading.Order, error) {
	nb.mu.RLock()
	defer nb.mu.RUnlock()

	order, ok := nb.orders[orderID]
	if !ok {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	return order, nil
}

// GetCash 获取现金
func (nb *NetworkBroker) GetCash() float64 {
	nb.mu.RLock()
	defer nb.mu.RUnlock()
	return nb.cash
}

// GetPositions 获取持仓
func (nb *NetworkBroker) GetPositions() map[string]*trading.Position {
	nb.mu.RLock()
	defer nb.mu.RUnlock()

	positions := make(map[string]*trading.Position)
	for k, v := range nb.positions {
		positions[k] = v
	}
	return positions
}
