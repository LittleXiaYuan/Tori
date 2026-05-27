package trading

import (
	"context"
	"time"
)

// ──────────────────────────────────────────────
// Core Types — Trading Engine 核心类型定义
// ──────────────────────────────────────────────

// SignalType 信号类型
type SignalType string

const (
	SignalBuy  SignalType = "buy"
	SignalSell SignalType = "sell"
	SignalHold SignalType = "hold"
)

// Signal 交易信号
type Signal struct {
	Type       SignalType `json:"type"`        // 买/卖/持有
	Stock      string     `json:"stock"`       // 股票代码 (e.g., "000001.SZ")
	Price      float64    `json:"price"`       // 建议价格
	Confidence float64    `json:"confidence"`  // 置信度 [0-1]
	Reason     string     `json:"reason"`      // 信号原因
	Strategy   string     `json:"strategy"`    // 生成信号的策略名称
	Timestamp  time.Time  `json:"timestamp"`   // 信号生成时间
	Metadata   map[string]any `json:"metadata,omitempty"` // 额外信息
}

// OrderAction 订单动作
type OrderAction string

const (
	OrderBuy  OrderAction = "buy"
	OrderSell OrderAction = "sell"
)

// Order 交易订单
type Order struct {
	ID        string      `json:"id"`         // 订单ID
	Action    OrderAction `json:"action"`     // 买/卖
	Stock     string      `json:"stock"`      // 股票代码
	Quantity  int         `json:"quantity"`   // 数量（股）
	Price     float64     `json:"price"`      // 价格
	Strategy  string      `json:"strategy"`   // 来源策略
	Timestamp time.Time   `json:"timestamp"`  // 下单时间
	Status    OrderStatus `json:"status"`     // 订单状态
	FilledQty int         `json:"filled_qty"` // 已成交数量
	AvgPrice  float64     `json:"avg_price"`  // 成交均价
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// OrderStatus 订单状态
type OrderStatus string

const (
	OrderPending   OrderStatus = "pending"    // 待提交
	OrderSubmitted OrderStatus = "submitted"  // 已提交
	OrderPartial   OrderStatus = "partial"    // 部分成交
	OrderFilled    OrderStatus = "filled"     // 全部成交
	OrderCancelled OrderStatus = "cancelled"  // 已撤销
	OrderRejected  OrderStatus = "rejected"   // 被拒绝
)

// Position 持仓
type Position struct {
	Stock        string    `json:"stock"`         // 股票代码
	Quantity     int       `json:"quantity"`      // 持仓数量
	AvgCost      float64   `json:"avg_cost"`      // 持仓成本
	CurrentPrice float64   `json:"current_price"` // 当前价格
	MarketValue  float64   `json:"market_value"`  // 市值
	PnL          float64   `json:"pnl"`           // 盈亏
	PnLPercent   float64   `json:"pnl_percent"`   // 盈亏比例
	UpdatedAt    time.Time `json:"updated_at"`    // 更新时间
}

// Portfolio 投资组合
type Portfolio struct {
	Cash       float64              `json:"cash"`        // 可用资金
	TotalValue float64              `json:"total_value"` // 总资产
	Positions  map[string]*Position `json:"positions"`   // 持仓列表 (stock -> position)
	UpdatedAt  time.Time            `json:"updated_at"`  // 更新时间
}

// Quote 行情数据
type Quote struct {
	Stock     string    `json:"stock"`      // 股票代码
	Price     float64   `json:"price"`      // 当前价格
	Open      float64   `json:"open"`       // 开盘价
	High      float64   `json:"high"`       // 最高价
	Low       float64   `json:"low"`        // 最低价
	Close     float64   `json:"close"`      // 收盘价
	Volume    int64     `json:"volume"`     // 成交量
	Timestamp time.Time `json:"timestamp"`  // 时间戳
	Change    float64   `json:"change"`     // 涨跌额
	ChangePct float64   `json:"change_pct"` // 涨跌幅
}

// KLine K线数据
type KLine struct {
	Stock     string    `json:"stock"`     // 股票代码
	Timestamp time.Time `json:"timestamp"` // 时间戳
	Open      float64   `json:"open"`      // 开盘价
	High      float64   `json:"high"`      // 最高价
	Low       float64   `json:"low"`       // 最低价
	Close     float64   `json:"close"`     // 收盘价
	Volume    int64     `json:"volume"`    // 成交量
	Amount    float64   `json:"amount"`    // 成交额
}

// Strategy 策略接口
type Strategy interface {
	// Name 策略名称
	Name() string

	// Analyze 分析行情，生成信号
	Analyze(ctx context.Context, stock string, klines []KLine) (*Signal, error)

	// Config 策略配置
	Config() map[string]any
}

// Broker 券商接口
type Broker interface {
	// Name 券商名称
	Name() string

	// GetQuote 获取实时行情
	GetQuote(ctx context.Context, stock string) (*Quote, error)

	// GetKLines 获取K线数据
	GetKLines(ctx context.Context, stock string, period string, count int) ([]KLine, error)

	// GetPortfolio 获取投资组合
	GetPortfolio(ctx context.Context) (*Portfolio, error)

	// SubmitOrder 提交订单
	SubmitOrder(ctx context.Context, order *Order) error

	// CancelOrder 撤销订单
	CancelOrder(ctx context.Context, orderID string) error

	// GetOrder 查询订单状态
	GetOrder(ctx context.Context, orderID string) (*Order, error)
}

// RiskCheck 风控检查结果
type RiskCheck struct {
	Passed bool     `json:"passed"` // 是否通过
	Reason string   `json:"reason"` // 拒绝原因
	Alerts []string `json:"alerts"` // 警告信息
}

// TradingConfig 交易配置
type TradingConfig struct {
	// 仓位控制
	MaxPositionSingle float64 `json:"max_position_single"` // 单只股票最大仓位比例 (0-1)
	MaxPositionTotal  float64 `json:"max_position_total"`  // 总仓位比例 (0-1)
	MaxIndustry       float64 `json:"max_industry"`        // 单行业最大仓位比例 (0-1)

	// 风控参数
	MaxDrawdown     float64 `json:"max_drawdown"`      // 最大回撤比例 (0-1)
	StopLoss        float64 `json:"stop_loss"`         // 默认止损比例 (0-1)
	TakeProfit      float64 `json:"take_profit"`       // 默认止盈比例 (0-1)
	MaxDailyLoss    float64 `json:"max_daily_loss"`    // 单日最大亏损比例 (0-1)
	MaxDailyTrades  int     `json:"max_daily_trades"`  // 单日最大交易次数
	VolatilityLimit float64 `json:"volatility_limit"`  // 波动率熔断阈值

	// 策略配置
	ActiveStrategies []string           `json:"active_strategies"` // 启用的策略列表
	StrategyWeights  map[string]float64 `json:"strategy_weights"`  // 策略权重
	SignalThreshold  float64            `json:"signal_threshold"`  // 信号阈值 (0-1)

	// 黑名单
	Blacklist []string `json:"blacklist"` // 黑名单股票

	// 其他
	Leverage int  `json:"leverage"` // 杠杆倍数
	DryRun   bool `json:"dry_run"`  // 模拟模式
}

// DefaultTradingConfig 默认配置
func DefaultTradingConfig() *TradingConfig {
	return &TradingConfig{
		MaxPositionSingle: 0.2,  // 单只股票最多20%
		MaxPositionTotal:  0.8,  // 总仓位最多80%
		MaxIndustry:       0.4,  // 单行业最多40%
		MaxDrawdown:       0.15, // 最大回撤15%
		StopLoss:          0.07, // 止损7%
		TakeProfit:        0.15, // 止盈15%
		MaxDailyLoss:      0.05, // 单日最大亏损5%
		MaxDailyTrades:    20,   // 单日最多20笔
		VolatilityLimit:   0.1,  // 波动率10%熔断
		ActiveStrategies:  []string{"ma_cross"},
		StrategyWeights: map[string]float64{
			"ma_cross": 1.0,
		},
		SignalThreshold: 0.6, // 信号置信度>0.6才执行
		Blacklist:       []string{},
		Leverage:        1,
		DryRun:          true, // 默认模拟模式
	}
}
