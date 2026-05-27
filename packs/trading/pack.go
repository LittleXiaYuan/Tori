package trading

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"yunque-agent/internal/agentcore/approval"
	"yunque-agent/internal/agentcore/trading"
	"yunque-agent/internal/cognikernel"
	"yunque-agent/packs/trading/brokers"
	"yunque-agent/packs/trading/market"
	tradingstrategies "yunque-agent/packs/trading/strategies"
)

// ──────────────────────────────────────────────
// TradingPack — 量化交易增量包
// 集成交易引擎、风控、审批、CogniKernel
// ──────────────────────────────────────────────

// Pack 量化交易包
type Pack struct {
	// 核心组件
	engine    *trading.Engine
	broker    trading.Broker
	portfolio *trading.PortfolioManager
	risk      *trading.RiskEngine
	config    *trading.TradingConfig

	// 集成组件
	approvalMgr *approval.Manager
	kernel      *cognikernel.Kernel

	// 运行状态
	running bool
}

// Config Pack配置
type Config struct {
	Broker              string  `json:"broker"`
	InitialCapital      float64 `json:"initial_capital"`
	MaxPositionSingle   float64 `json:"max_position_single"`
	MaxPositionTotal    float64 `json:"max_position_total"`
	StopLoss            float64 `json:"stop_loss"`
	TakeProfit          float64 `json:"take_profit"`
	MaxDailyLoss        float64 `json:"max_daily_loss"`
	ActiveStrategies    []string `json:"active_strategies"`
	AutoApproveThreshold float64 `json:"auto_approve_threshold"`
}

// NewPack 创建交易包
func NewPack(cfg *Config, approvalMgr *approval.Manager, kernel *cognikernel.Kernel) (*Pack, error) {
	if cfg == nil {
		cfg = &Config{
			Broker:            "network",
			InitialCapital:    100000,
			MaxPositionSingle: 0.2,
			MaxPositionTotal:  0.8,
			StopLoss:          0.07,
			TakeProfit:        0.15,
			MaxDailyLoss:      0.05,
			ActiveStrategies:  []string{"ma_cross"},
			AutoApproveThreshold: 0,
		}
	}

	// 创建券商
	var broker trading.Broker
	switch cfg.Broker {
	case "simulate":
		broker = trading.NewSimulateBroker(cfg.InitialCapital)
	case "network":
		// 使用真实行情 + 模拟账户
		provider := market.NewSinaProvider()
		broker = brokers.NewNetworkBroker("network", cfg.InitialCapital, provider)
	default:
		return nil, fmt.Errorf("unsupported broker: %s", cfg.Broker)
	}

	// 创建交易配置
	tradingConfig := &trading.TradingConfig{
		MaxPositionSingle: cfg.MaxPositionSingle,
		MaxPositionTotal:  cfg.MaxPositionTotal,
		StopLoss:          cfg.StopLoss,
		TakeProfit:        cfg.TakeProfit,
		MaxDailyLoss:      cfg.MaxDailyLoss,
		ActiveStrategies:  cfg.ActiveStrategies,
		DryRun:            false, // 真实模式
	}

	// 创建引擎
	engine := trading.NewEngine(broker, tradingConfig)

	// 创建组件
	portfolio := trading.NewPortfolioManager(broker, tradingConfig)
	risk := trading.NewRiskEngine(broker, tradingConfig, portfolio)

	engine.SetPortfolioManager(portfolio)
	engine.SetRiskEngine(risk)

	// 策略注册已移至Cogni声明式配置
	// 通过Cogni的context和surface动态加载策略
	// 不再硬编码策略列表

	pack := &Pack{
		engine:      engine,
		broker:      broker,
		portfolio:   portfolio,
		risk:        risk,
		config:      tradingConfig,
		approvalMgr: approvalMgr,
		kernel:      kernel,
	}

	// 设置事件回调
	pack.setupCallbacks()

	return pack, nil
}

// setupCallbacks 设置事件回调
func (p *Pack) setupCallbacks() {
	// 信号回调
	p.engine.OnSignal(func(signal *trading.Signal) {
		// 发送到 CogniKernel
		if p.kernel != nil {
			p.kernel.PublishEvent(cognikernel.Event{
				Type: "strategy_signal",
				Data: map[string]any{
					"stock":      signal.Stock,
					"type":       signal.Type,
					"confidence": signal.Confidence,
					"reason":     signal.Reason,
					"strategy":   signal.Strategy,
				},
			})
		}
	})

	// 订单回调
	p.engine.OnOrder(func(order *trading.Order) {
		slog.Info("trading: order submitted",
			"id", order.ID,
			"stock", order.Stock,
			"action", order.Action,
			"quantity", order.Quantity)
	})

	// 成交回调
	p.engine.OnTrade(func(order *trading.Order) {
		slog.Info("trading: trade executed",
			"stock", order.Stock,
			"action", order.Action,
			"quantity", order.Quantity,
			"price", order.AvgPrice)

		// 发送到 CogniKernel 进行反思
		if p.kernel != nil {
			p.kernel.PublishEvent(cognikernel.Event{
				Type: "trade_executed",
				Data: map[string]any{
					"stock":    order.Stock,
					"action":   order.Action,
					"quantity": order.Quantity,
					"price":    order.AvgPrice,
					"strategy": order.Strategy,
				},
			})

			// 记录经验
			p.kernel.IngestFeedback(cognikernel.Feedback{
				Category: "trading",
				Outcome:  "executed",
				Lesson:   fmt.Sprintf("%s %s @ %.2f", order.Action, order.Stock, order.AvgPrice),
				Context:  fmt.Sprintf("strategy: %s, confidence: %.2f", order.Strategy, order.Metadata["confidence"]),
			})
		}
	})

	// 风控警报回调
	p.engine.OnRiskAlert(func(reason string) {
		slog.Warn("trading: risk alert", "reason", reason)

		// 发送到 CogniKernel
		if p.kernel != nil {
			p.kernel.PublishEvent(cognikernel.Event{
				Type: "risk_alert",
				Data: map[string]any{
					"reason": reason,
				},
			})
		}
	})
}

// Start 启动交易包
func (p *Pack) Start(ctx context.Context) error {
	if p.running {
		return fmt.Errorf("already running")
	}

	if err := p.engine.Start(ctx); err != nil {
		return fmt.Errorf("start engine: %w", err)
	}

	p.running = true
	slog.Info("trading pack started")
	return nil
}

// Stop 停止交易包
func (p *Pack) Stop() {
	if !p.running {
		return
	}

	p.engine.Stop()
	p.running = false
	slog.Info("trading pack stopped")
}

// ──────────────────────────────────────────────
// Skills — 技能实现
// ──────────────────────────────────────────────

// AnalyzeStock 分析股票
// stock参数应从用户输入中提取，不再硬编码
func (p *Pack) AnalyzeStock(ctx context.Context, stock string) (map[string]any, error) {
	if stock == "" {
		return nil, fmt.Errorf("股票代码不能为空，请提供如 000001.SZ 格式的代码")
	}

	signals, err := p.engine.AnalyzeStock(ctx, stock)
	if err != nil {
		return nil, fmt.Errorf("analyze stock: %w", err)
	}

	result := map[string]any{
		"stock":   stock,
		"signals": signals,
		"count":   len(signals),
	}

	return result, nil
}

// GetPortfolio 获取持仓
func (p *Pack) GetPortfolio(ctx context.Context) (map[string]any, error) {
	portfolio, err := p.engine.GetPortfolio(ctx)
	if err != nil {
		return nil, fmt.Errorf("get portfolio: %w", err)
	}

	result := map[string]any{
		"cash":        portfolio.Cash,
		"total_value": portfolio.TotalValue,
		"positions":   portfolio.Positions,
		"updated_at":  portfolio.UpdatedAt,
	}

	return result, nil
}

// GetQuote 获取实时行情
func (p *Pack) GetQuote(ctx context.Context, stock string) (map[string]any, error) {
	quote, err := p.broker.GetQuote(ctx, stock)
	if err != nil {
		return nil, fmt.Errorf("get quote: %w", err)
	}

	result := map[string]any{
		"stock":      quote.Stock,
		"price":      quote.Price,
		"open":       quote.Open,
		"high":       quote.High,
		"low":        quote.Low,
		"volume":     quote.Volume,
		"change":     quote.Change,
		"change_pct": quote.ChangePct,
		"timestamp":  quote.Timestamp,
	}

	return result, nil
}

// ExecuteTrade 执行交易（需要审批）
func (p *Pack) ExecuteTrade(ctx context.Context, stock string, action string, quantity int) (map[string]any, error) {
	// 创建订单
	order := &trading.Order{
		ID:        fmt.Sprintf("order_%s_%d", stock, time.Now().Unix()),
		Action:    trading.OrderAction(action),
		Stock:     stock,
		Quantity:  quantity,
		Timestamp: time.Now(),
		Status:    trading.OrderPending,
	}

	// 获取当前价格
	quote, err := p.broker.GetQuote(ctx, stock)
	if err != nil {
		return nil, fmt.Errorf("get quote: %w", err)
	}
	order.Price = quote.Price

	// 创建审批请求
	if p.approvalMgr != nil {
		orderJSON, _ := json.Marshal(order)
		req := &approval.Request{
			Category:  approval.CatFinancial,
			RiskLevel: approval.RiskHigh,
			Summary:   fmt.Sprintf("%s %s × %d股 @ ¥%.2f", action, stock, quantity, quote.Price),
			Details:   string(orderJSON),
		}

		// 提交审批
		approved, err := p.approvalMgr.RequestApproval(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("request approval: %w", err)
		}

		if !approved {
			return map[string]any{
				"status":  "rejected",
				"message": "交易被拒绝",
			}, nil
		}
	}

	// 执行订单
	if err := p.engine.SubmitOrder(ctx, order); err != nil {
		return nil, fmt.Errorf("submit order: %w", err)
	}

	result := map[string]any{
		"status":    "success",
		"order_id":  order.ID,
		"stock":     order.Stock,
		"action":    order.Action,
		"quantity":  order.Quantity,
		"price":     order.AvgPrice,
		"filled_qty": order.FilledQty,
	}

	return result, nil
}

// BacktestStrategy 回测策略
func (p *Pack) BacktestStrategy(ctx context.Context, strategyName string, stock string, startDate string, endDate string) (map[string]any, error) {
	// TODO: 实现回测功能
	return map[string]any{
		"status":  "not_implemented",
		"message": "回测功能开发中",
	}, nil
}
