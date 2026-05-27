package trading

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// ──────────────────────────────────────────────
// Engine — Trading Engine 核心引擎
// 统筹策略→风控→执行的完整流程
// ──────────────────────────────────────────────

// Engine 交易引擎
type Engine struct {
	mu sync.RWMutex

	// 核心组件
	broker     Broker
	strategies map[string]Strategy
	portfolio  *PortfolioManager
	risk       *RiskEngine
	config     *TradingConfig

	// 信号缓冲
	signalBuffer []*Signal

	// 运行状态
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	// 事件回调
	onSignal   func(*Signal)
	onOrder    func(*Order)
	onTrade    func(*Order)
	onRiskAlert func(string)
}

// NewEngine 创建交易引擎
func NewEngine(broker Broker, config *TradingConfig) *Engine {
	if config == nil {
		config = DefaultTradingConfig()
	}

	return &Engine{
		broker:       broker,
		strategies:   make(map[string]Strategy),
		config:       config,
		signalBuffer: make([]*Signal, 0, 100),
	}
}

// RegisterStrategy 注册策略
func (e *Engine) RegisterStrategy(strategy Strategy) {
	e.mu.Lock()
	defer e.mu.Unlock()

	name := strategy.Name()
	e.strategies[name] = strategy
	slog.Info("trading: strategy registered", "name", name)
}

// SetPortfolioManager 设置投资组合管理器
func (e *Engine) SetPortfolioManager(pm *PortfolioManager) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.portfolio = pm
}

// SetRiskEngine 设置风控引擎
func (e *Engine) SetRiskEngine(re *RiskEngine) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.risk = re
}

// OnSignal 注册信号回调
func (e *Engine) OnSignal(fn func(*Signal)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onSignal = fn
}

// OnOrder 注册订单回调
func (e *Engine) OnOrder(fn func(*Order)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onOrder = fn
}

// OnTrade 注册成交回调
func (e *Engine) OnTrade(fn func(*Order)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onTrade = fn
}

// OnRiskAlert 注册风控警报回调
func (e *Engine) OnRiskAlert(fn func(string)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onRiskAlert = fn
}

// Start 启动引擎
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("engine already running")
	}

	e.ctx, e.cancel = context.WithCancel(ctx)
	e.running = true
	e.mu.Unlock()

	slog.Info("trading: engine started", "dry_run", e.config.DryRun)
	return nil
}

// Stop 停止引擎
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}

	if e.cancel != nil {
		e.cancel()
	}
	e.running = false
	slog.Info("trading: engine stopped")
}

// IsRunning 是否运行中
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// AnalyzeStock 分析股票，生成信号
func (e *Engine) AnalyzeStock(ctx context.Context, stock string) ([]*Signal, error) {
	e.mu.RLock()
	activeStrategies := e.config.ActiveStrategies
	strategies := make(map[string]Strategy)
	for _, name := range activeStrategies {
		if s, ok := e.strategies[name]; ok {
			strategies[name] = s
		}
	}
	e.mu.RUnlock()

	if len(strategies) == 0 {
		return nil, fmt.Errorf("no active strategies")
	}

	// 获取K线数据
	klines, err := e.broker.GetKLines(ctx, stock, "1d", 100)
	if err != nil {
		return nil, fmt.Errorf("get klines: %w", err)
	}

	// 每个策略独立分析
	signals := make([]*Signal, 0, len(strategies))
	for name, strategy := range strategies {
		signal, err := strategy.Analyze(ctx, stock, klines)
		if err != nil {
			slog.Warn("trading: strategy analyze failed", "strategy", name, "stock", stock, "err", err)
			continue
		}

		if signal != nil && signal.Type != SignalHold {
			signals = append(signals, signal)
			slog.Info("trading: signal generated",
				"strategy", name,
				"stock", stock,
				"type", signal.Type,
				"confidence", signal.Confidence,
				"reason", signal.Reason)

			// 触发回调
			e.mu.RLock()
			if e.onSignal != nil {
				e.onSignal(signal)
			}
			e.mu.RUnlock()
		}
	}

	return signals, nil
}

// ExecuteSignals 执行信号（组合决策 + 风控 + 下单）
func (e *Engine) ExecuteSignals(ctx context.Context, signals []*Signal) error {
	if len(signals) == 0 {
		return nil
	}

	e.mu.RLock()
	portfolio := e.portfolio
	risk := e.risk
	e.mu.RUnlock()

	if portfolio == nil {
		return fmt.Errorf("portfolio manager not set")
	}
	if risk == nil {
		return fmt.Errorf("risk engine not set")
	}

	// 1. 组合决策：信号加权汇总
	orders, err := portfolio.DecideOrders(ctx, signals)
	if err != nil {
		return fmt.Errorf("decide orders: %w", err)
	}

	// 2. 风控检查
	for _, order := range orders {
		check := risk.CheckOrder(ctx, order)
		if !check.Passed {
			slog.Warn("trading: order rejected by risk engine",
				"stock", order.Stock,
				"action", order.Action,
				"reason", check.Reason)

			// 触发风控警报
			e.mu.RLock()
			if e.onRiskAlert != nil {
				e.onRiskAlert(check.Reason)
			}
			e.mu.RUnlock()
			continue
		}

		// 3. 提交订单
		if err := e.SubmitOrder(ctx, order); err != nil {
			slog.Error("trading: submit order failed",
				"stock", order.Stock,
				"action", order.Action,
				"err", err)
			continue
		}
	}

	return nil
}

// SubmitOrder 提交订单
func (e *Engine) SubmitOrder(ctx context.Context, order *Order) error {
	e.mu.RLock()
	dryRun := e.config.DryRun
	e.mu.RUnlock()

	if dryRun {
		slog.Info("trading: [DRY RUN] order submitted",
			"stock", order.Stock,
			"action", order.Action,
			"quantity", order.Quantity,
			"price", order.Price)
		order.Status = OrderFilled
		order.FilledQty = order.Quantity
		order.AvgPrice = order.Price

		// 触发成交回调
		e.mu.RLock()
		if e.onTrade != nil {
			e.onTrade(order)
		}
		e.mu.RUnlock()
		return nil
	}

	// 实盘提交
	if err := e.broker.SubmitOrder(ctx, order); err != nil {
		return fmt.Errorf("broker submit order: %w", err)
	}

	slog.Info("trading: order submitted",
		"id", order.ID,
		"stock", order.Stock,
		"action", order.Action,
		"quantity", order.Quantity,
		"price", order.Price)

	// 触发订单回调
	e.mu.RLock()
	if e.onOrder != nil {
		e.onOrder(order)
	}
	e.mu.RUnlock()

	return nil
}

// GetPortfolio 获取投资组合
func (e *Engine) GetPortfolio(ctx context.Context) (*Portfolio, error) {
	return e.broker.GetPortfolio(ctx)
}

// GetConfig 获取配置
func (e *Engine) GetConfig() *TradingConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config
}

// UpdateConfig 更新配置
func (e *Engine) UpdateConfig(config *TradingConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = config
	slog.Info("trading: config updated")
}

// GetStrategies 获取所有策略
func (e *Engine) GetStrategies() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	names := make([]string, 0, len(e.strategies))
	for name := range e.strategies {
		names = append(names, name)
	}
	return names
}

// GetActiveStrategies 获取启用的策略
func (e *Engine) GetActiveStrategies() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config.ActiveStrategies
}

// SetActiveStrategies 设置启用的策略
func (e *Engine) SetActiveStrategies(strategies []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config.ActiveStrategies = strategies
	slog.Info("trading: active strategies updated", "strategies", strategies)
}

// GetBroker 获取券商
func (e *Engine) GetBroker() Broker {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.broker
}
