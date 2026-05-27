package trading_test

import (
	"context"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/trading"
	"yunque-agent/internal/agentcore/trading/brokers"
	"yunque-agent/internal/agentcore/trading/strategies"
)

// TestTradingEngine_BasicFlow 测试基本交易流程
func TestTradingEngine_BasicFlow(t *testing.T) {
	ctx := context.Background()

	// 1. 创建模拟券商（初始资金10万）
	broker := brokers.NewSimulateBroker(100000)

	// 2. 创建配置
	config := trading.DefaultTradingConfig()
	config.DryRun = false // 使用模拟券商，不是dry run

	// 3. 创建交易引擎
	engine := trading.NewEngine(broker, config)

	// 4. 创建投资组合管理器
	portfolio := trading.NewPortfolioManager(broker, config)
	engine.SetPortfolioManager(portfolio)

	// 5. 创建风控引擎
	risk := trading.NewRiskEngine(broker, config, portfolio)
	engine.SetRiskEngine(risk)

	// 6. 注册策略
	strategy := strategies.NewMACrossStrategy(5, 20)
	engine.RegisterStrategy(strategy)

	// 7. 启动引擎
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("start engine: %v", err)
	}
	defer engine.Stop()

	// 测试用例：使用测试股票代码
	// 注意：实际使用时股票代码应从用户输入或Cogni上下文中获取
	stock := "TEST001.SZ"
	klines := generateGoldenCrossKLines(stock, 30)
	broker.SetKLines(stock, klines)

	// 设置当前行情
	broker.SetQuote(stock, &trading.Quote{
		Stock:     stock,
		Price:     12.5,
		Open:      12.0,
		High:      12.8,
		Low:       11.8,
		Close:     12.5,
		Volume:    1000000,
		Timestamp: time.Now(),
		Change:    0.5,
		ChangePct: 0.042,
	})

	// 9. 分析股票
	t.Logf("Analyzing stock %s with %d klines", stock, len(klines))

	// 打印最后几根K线
	for i := len(klines) - 5; i < len(klines); i++ {
		t.Logf("KLine[%d]: Close=%.2f", i, klines[i].Close)
	}

	signals, err := engine.AnalyzeStock(ctx, stock)
	if err != nil {
		t.Fatalf("analyze stock: %v", err)
	}

	t.Logf("Got %d signals", len(signals))

	if len(signals) == 0 {
		t.Fatal("expected signals, got none")
	}

	t.Logf("Generated %d signals", len(signals))
	for _, sig := range signals {
		t.Logf("Signal: %s %s @ %.2f (confidence: %.2f, reason: %s)",
			sig.Type, sig.Stock, sig.Price, sig.Confidence, sig.Reason)
	}

	// 10. 执行信号
	if err := engine.ExecuteSignals(ctx, signals); err != nil {
		t.Fatalf("execute signals: %v", err)
	}

	// 11. 验证结果
	portfolioResult, err := engine.GetPortfolio(ctx)
	if err != nil {
		t.Fatalf("get portfolio: %v", err)
	}

	t.Logf("Portfolio: Cash=%.2f, TotalValue=%.2f, Positions=%d",
		portfolioResult.Cash, portfolioResult.TotalValue, len(portfolioResult.Positions))

	// 应该有持仓
	if len(portfolioResult.Positions) == 0 {
		t.Error("expected positions, got none")
	}

	// 验证持仓
	if pos, ok := portfolioResult.Positions[stock]; ok {
		t.Logf("Position: %s Qty=%d AvgCost=%.2f CurrentPrice=%.2f PnL=%.2f (%.2f%%)",
			pos.Stock, pos.Quantity, pos.AvgCost, pos.CurrentPrice, pos.PnL, pos.PnLPercent*100)

		if pos.Quantity <= 0 {
			t.Error("expected positive quantity")
		}
	} else {
		t.Errorf("expected position for %s", stock)
	}
}

// TestRiskEngine_PositionLimit 测试仓位限制
func TestRiskEngine_PositionLimit(t *testing.T) {
	ctx := context.Background()

	broker := brokers.NewSimulateBroker(100000)
	config := trading.DefaultTradingConfig()
	config.MaxPositionSingle = 0.2 // 单只股票最多20%

	portfolio := trading.NewPortfolioManager(broker, config)
	risk := trading.NewRiskEngine(broker, config, portfolio)

	// 刷新组合
	if err := portfolio.RefreshPortfolio(ctx); err != nil {
		t.Fatalf("refresh portfolio: %v", err)
	}

	stock := "TEST001.SZ"
	broker.SetQuote(stock, &trading.Quote{
		Stock: stock,
		Price: 10.0,
	})

	// 尝试买入超过限制的数量
	order := &trading.Order{
		Action:   trading.OrderBuy,
		Stock:    stock,
		Quantity: 3000, // 3000股 × 10元 = 30000元 = 30% > 20%
		Price:    10.0,
	}

	check := risk.CheckOrder(ctx, order)
	if check.Passed {
		t.Error("expected order to be rejected due to position limit")
	}

	t.Logf("Order rejected: %s", check.Reason)
}

// TestRiskEngine_CircuitBreaker 测试熔断器
func TestRiskEngine_CircuitBreaker(t *testing.T) {
	ctx := context.Background()

	broker := brokers.NewSimulateBroker(100000)
	config := trading.DefaultTradingConfig()
	config.MaxDailyLoss = 0.05 // 单日最大亏损5%

	portfolio := trading.NewPortfolioManager(broker, config)
	risk := trading.NewRiskEngine(broker, config, portfolio)

	// 模拟连续亏损
	for i := 0; i < 3; i++ {
		risk.RecordTrade(&trading.Order{}, -1000)
	}

	// 熔断器应该被触发
	cb := risk.GetCircuitBreaker()
	if !cb.IsTripped() {
		t.Error("expected circuit breaker to be tripped")
	}

	t.Logf("Circuit breaker tripped: %s", cb.TripReason())

	// 尝试下单应该被拒绝
	if err := portfolio.RefreshPortfolio(ctx); err != nil {
		t.Fatalf("refresh portfolio: %v", err)
	}

	order := &trading.Order{
		Action:   trading.OrderBuy,
		Stock:    "000001.SZ",
		Quantity: 100,
		Price:    10.0,
	}

	check := risk.CheckOrder(ctx, order)
	if check.Passed {
		t.Error("expected order to be rejected due to circuit breaker")
	}
}

// TestMACrossStrategy_GoldenCross 测试均线金叉
func TestMACrossStrategy_GoldenCross(t *testing.T) {
	ctx := context.Background()
	strategy := strategies.NewMACrossStrategy(5, 20)

	stock := "TEST001.SZ"
	klines := generateGoldenCrossKLines(stock, 30)

	// 打印均线值用于调试
	shortMA := calculateMA(klines, 5, 0)
	longMA := calculateMA(klines, 20, 0)
	prevShortMA := calculateMA(klines, 5, 1)
	prevLongMA := calculateMA(klines, 20, 1)

	t.Logf("Current: Short MA=%.2f, Long MA=%.2f", shortMA, longMA)
	t.Logf("Previous: Short MA=%.2f, Long MA=%.2f", prevShortMA, prevLongMA)

	signal, err := strategy.Analyze(ctx, stock, klines)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	if signal == nil {
		t.Fatal("expected signal, got nil (no golden cross detected)")
	}

	if signal.Type != trading.SignalBuy {
		t.Errorf("expected buy signal, got %s", signal.Type)
	}

	t.Logf("Signal: %s (confidence: %.2f, reason: %s)",
		signal.Type, signal.Confidence, signal.Reason)
}

// calculateMA 辅助函数：计算移动平均线
func calculateMA(klines []trading.KLine, period int, offset int) float64 {
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

// TestMACrossStrategy_DeathCross 测试均线死叉
func TestMACrossStrategy_DeathCross(t *testing.T) {
	ctx := context.Background()
	strategy := strategies.NewMACrossStrategy(5, 20)

	stock := "TEST001.SZ"
	klines := generateDeathCrossKLines(stock, 30)

	// 打印均线值用于调试
	shortMA := calculateMA(klines, 5, 0)
	longMA := calculateMA(klines, 20, 0)
	prevShortMA := calculateMA(klines, 5, 1)
	prevLongMA := calculateMA(klines, 20, 1)

	t.Logf("Current: Short MA=%.2f, Long MA=%.2f", shortMA, longMA)
	t.Logf("Previous: Short MA=%.2f, Long MA=%.2f", prevShortMA, prevLongMA)

	signal, err := strategy.Analyze(ctx, stock, klines)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	if signal == nil {
		t.Fatal("expected signal, got nil (no death cross detected)")
	}

	if signal.Type != trading.SignalSell {
		t.Errorf("expected sell signal, got %s", signal.Type)
	}

	t.Logf("Signal: %s (confidence: %.2f, reason: %s)",
		signal.Type, signal.Confidence, signal.Reason)
}

// generateGoldenCrossKLines 生成金叉场景的K线数据
func generateGoldenCrossKLines(stock string, count int) []trading.KLine {
	klines := make([]trading.KLine, count)
	now := time.Now()

	// 生成一个明确的金叉场景：
	// 前25根：短期均线在长期均线下方
	// 最后5根：快速上涨，短期均线穿过长期均线

	for i := 0; i < count; i++ {
		var price float64

		if i < 20 {
			// 前20根：缓慢下跌，短期均线在长期均线下方
			price = 12.0 - float64(i)*0.08
		} else if i < 28 {
			// 21-27根：横盘，短期均线接近但仍在长期均线下方
			price = 10.4 + float64(i-20)*0.02
		} else {
			// 最后2根：快速上涨，形成金叉
			price = 10.54 + float64(i-27)*0.5
		}

		klines[i] = trading.KLine{
			Stock:     stock,
			Timestamp: now.Add(-time.Duration(count-i) * 24 * time.Hour),
			Open:      price,
			High:      price * 1.01,
			Low:       price * 0.99,
			Close:     price,
			Volume:    1000000 + int64(i)*50000,
			Amount:    price * float64(1000000+int64(i)*50000),
		}
	}

	return klines
}

// generateDeathCrossKLines 生成死叉场景的K线数据
func generateDeathCrossKLines(stock string, count int) []trading.KLine {
	klines := make([]trading.KLine, count)
	now := time.Now()

	// 生成一个明确的死叉场景：
	// 前25根：短期均线在长期均线上方
	// 最后5根：快速下跌，短期均线穿过长期均线

	for i := 0; i < count; i++ {
		var price float64

		if i < 20 {
			// 前20根：缓慢上涨，短期均线在长期均线上方
			price = 10.0 + float64(i)*0.08
		} else if i < 28 {
			// 21-27根：横盘，短期均线接近但仍在长期均线上方
			price = 11.6 - float64(i-20)*0.02
		} else {
			// 最后2根：快速下跌，形成死叉
			price = 11.46 - float64(i-27)*0.5
		}

		klines[i] = trading.KLine{
			Stock:     stock,
			Timestamp: now.Add(-time.Duration(count-i) * 24 * time.Hour),
			Open:      price,
			High:      price * 1.01,
			Low:       price * 0.99,
			Close:     price,
			Volume:    1000000 + int64(i)*50000,
			Amount:    price * float64(1000000+int64(i)*50000),
		}
	}

	return klines
}
