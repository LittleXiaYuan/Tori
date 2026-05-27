package brokers

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/trading"
)

// ──────────────────────────────────────────────
// SimulateBroker — 模拟券商
// 用于测试和回测，不连接真实券商
// ──────────────────────────────────────────────

// SimulateBroker 模拟券商
type SimulateBroker struct {
	mu sync.RWMutex

	// 模拟账户
	cash       float64
	positions  map[string]*trading.Position
	totalValue float64

	// 模拟行情数据
	quotes map[string]*trading.Quote
	klines map[string][]trading.KLine

	// 订单记录
	orders map[string]*trading.Order
}

// NewSimulateBroker 创建模拟券商
func NewSimulateBroker(initialCash float64) *SimulateBroker {
	return &SimulateBroker{
		cash:       initialCash,
		positions:  make(map[string]*trading.Position),
		totalValue: initialCash,
		quotes:     make(map[string]*trading.Quote),
		klines:     make(map[string][]trading.KLine),
		orders:     make(map[string]*trading.Order),
	}
}

// Name 券商名称
func (sb *SimulateBroker) Name() string {
	return "simulate"
}

// GetQuote 获取实时行情
func (sb *SimulateBroker) GetQuote(ctx context.Context, stock string) (*trading.Quote, error) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	// 如果有预设行情，返回预设
	if quote, ok := sb.quotes[stock]; ok {
		return quote, nil
	}

	// 否则生成随机行情
	basePrice := 10.0 + rand.Float64()*90.0 // 10-100元
	change := (rand.Float64() - 0.5) * 0.2  // ±10%

	quote := &trading.Quote{
		Stock:     stock,
		Price:     basePrice * (1 + change),
		Open:      basePrice,
		High:      basePrice * (1 + abs(change)),
		Low:       basePrice * (1 - abs(change)),
		Close:     basePrice * (1 + change),
		Volume:    int64(rand.Intn(10000000)),
		Timestamp: time.Now(),
		Change:    basePrice * change,
		ChangePct: change,
	}

	return quote, nil
}

// GetKLines 获取K线数据
func (sb *SimulateBroker) GetKLines(ctx context.Context, stock string, period string, count int) ([]trading.KLine, error) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	// 如果有预设K线，返回预设
	if klines, ok := sb.klines[stock]; ok {
		if len(klines) > count {
			return klines[len(klines)-count:], nil
		}
		return klines, nil
	}

	// 否则生成模拟K线
	klines := make([]trading.KLine, count)
	basePrice := 10.0 + rand.Float64()*90.0
	currentPrice := basePrice

	now := time.Now()
	for i := 0; i < count; i++ {
		// 随机游走
		change := (rand.Float64() - 0.5) * 0.05 // ±2.5%
		open := currentPrice
		close := currentPrice * (1 + change)
		high := max(open, close) * (1 + rand.Float64()*0.02)
		low := min(open, close) * (1 - rand.Float64()*0.02)

		klines[i] = trading.KLine{
			Stock:     stock,
			Timestamp: now.Add(-time.Duration(count-i) * 24 * time.Hour),
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    int64(rand.Intn(10000000)),
			Amount:    close * float64(rand.Intn(10000000)),
		}

		currentPrice = close
	}

	return klines, nil
}

// GetPortfolio 获取投资组合
func (sb *SimulateBroker) GetPortfolio(ctx context.Context) (*trading.Portfolio, error) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	// 更新持仓市值
	totalValue := sb.cash
	positions := make(map[string]*trading.Position)

	for stock, pos := range sb.positions {
		quote, err := sb.GetQuote(ctx, stock)
		if err != nil {
			continue
		}

		currentPrice := quote.Price
		marketValue := float64(pos.Quantity) * currentPrice
		pnl := marketValue - float64(pos.Quantity)*pos.AvgCost
		pnlPercent := pnl / (float64(pos.Quantity) * pos.AvgCost)

		newPos := &trading.Position{
			Stock:        stock,
			Quantity:     pos.Quantity,
			AvgCost:      pos.AvgCost,
			CurrentPrice: currentPrice,
			MarketValue:  marketValue,
			PnL:          pnl,
			PnLPercent:   pnlPercent,
			UpdatedAt:    time.Now(),
		}

		positions[stock] = newPos
		totalValue += marketValue
	}

	return &trading.Portfolio{
		Cash:       sb.cash,
		TotalValue: totalValue,
		Positions:  positions,
		UpdatedAt:  time.Now(),
	}, nil
}

// SubmitOrder 提交订单
func (sb *SimulateBroker) SubmitOrder(ctx context.Context, order *trading.Order) error {
	// 先获取价格（在锁外）
	quote, err := sb.GetQuote(ctx, order.Stock)
	if err != nil {
		return fmt.Errorf("get quote: %w", err)
	}

	price := quote.Price
	totalCost := float64(order.Quantity) * price

	// 再获取锁进行账户操作
	sb.mu.Lock()
	defer sb.mu.Unlock()

	switch order.Action {
	case trading.OrderBuy:
		// 检查资金
		if totalCost > sb.cash {
			order.Status = trading.OrderRejected
			return fmt.Errorf("insufficient cash: need %.2f, have %.2f", totalCost, sb.cash)
		}

		// 扣除资金
		sb.cash -= totalCost

		// 更新持仓
		if pos, ok := sb.positions[order.Stock]; ok {
			// 已有持仓，计算新的平均成本
			totalQty := pos.Quantity + order.Quantity
			totalCostBasis := float64(pos.Quantity)*pos.AvgCost + totalCost
			pos.Quantity = totalQty
			pos.AvgCost = totalCostBasis / float64(totalQty)
		} else {
			// 新建持仓
			sb.positions[order.Stock] = &trading.Position{
				Stock:    order.Stock,
				Quantity: order.Quantity,
				AvgCost:  price,
			}
		}

	case trading.OrderSell:
		// 检查持仓
		pos, ok := sb.positions[order.Stock]
		if !ok || pos.Quantity < order.Quantity {
			order.Status = trading.OrderRejected
			return fmt.Errorf("insufficient position")
		}

		// 增加资金
		sb.cash += totalCost

		// 更新持仓
		pos.Quantity -= order.Quantity
		if pos.Quantity == 0 {
			delete(sb.positions, order.Stock)
		}
	}

	// 更新订单状态
	order.Status = trading.OrderFilled
	order.FilledQty = order.Quantity
	order.AvgPrice = price
	sb.orders[order.ID] = order

	return nil
}

// CancelOrder 撤销订单
func (sb *SimulateBroker) CancelOrder(ctx context.Context, orderID string) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	order, ok := sb.orders[orderID]
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
func (sb *SimulateBroker) GetOrder(ctx context.Context, orderID string) (*trading.Order, error) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	order, ok := sb.orders[orderID]
	if !ok {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	return order, nil
}

// SetQuote 设置行情（用于测试）
func (sb *SimulateBroker) SetQuote(stock string, quote *trading.Quote) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.quotes[stock] = quote
}

// SetKLines 设置K线数据（用于测试）
func (sb *SimulateBroker) SetKLines(stock string, klines []trading.KLine) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.klines[stock] = klines
}

// GetCash 获取现金
func (sb *SimulateBroker) GetCash() float64 {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.cash
}

// GetPositions 获取持仓
func (sb *SimulateBroker) GetPositions() map[string]*trading.Position {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	positions := make(map[string]*trading.Position)
	for k, v := range sb.positions {
		positions[k] = v
	}
	return positions
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
