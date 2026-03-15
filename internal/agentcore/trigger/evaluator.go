package trigger

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// ──────────────────────────────────────────────
// ConditionEvaluator — 条件评估引擎
//
// 支持的检查类型：
// - task_status:  目标任务状态匹配
// - cost_threshold: 当日/当月成本超阈值
// - memory_count: 记忆条目数超阈值
// - custom:       自定义 "key op value" 表达式
//
// 比较运算符："eq", "neq", "gt", "lt", "gte", "lte", "contains"
// ──────────────────────────────────────────────

// DataSource 条件评估所需的外部数据源
type DataSource struct {
	// GetTaskStatus returns task status by ID ("pending","running","completed","failed","paused","interrupted","cancelled")
	GetTaskStatus func(taskID string) (string, error)

	// GetTodayCost returns today's total cost in USD
	GetTodayCost func() float64

	// GetMonthCost returns this month's total cost in USD
	GetMonthCost func() float64

	// GetMemoryCount returns total memory items for a tenant
	GetMemoryCount func(tenantID string) int

	// GetCustomValue returns a custom metric value by key
	GetCustomValue func(key string) (string, error)
}

// NewConditionEvaluator creates a ConditionEvaluator backed by the given DataSource.
func NewConditionEvaluator(ds *DataSource) ConditionEvaluator {
	return func(ctx context.Context, config *ConditionConfig) (bool, error) {
		if config == nil {
			return false, fmt.Errorf("nil condition config")
		}

		var actual string
		var err error

		switch config.CheckType {
		case "task_status":
			if ds.GetTaskStatus == nil {
				return false, fmt.Errorf("GetTaskStatus not configured")
			}
			if config.TargetID == "" {
				return false, fmt.Errorf("target_id required for task_status check")
			}
			actual, err = ds.GetTaskStatus(config.TargetID)
			if err != nil {
				return false, err
			}

		case "cost_threshold":
			// Value format: "day:10.5" or "month:100" or just "10.5" (defaults to day)
			var costVal float64
			if ds.GetTodayCost != nil && ds.GetMonthCost != nil {
				if strings.HasPrefix(config.TargetID, "month") {
					costVal = ds.GetMonthCost()
				} else {
					costVal = ds.GetTodayCost()
				}
			}
			actual = fmt.Sprintf("%.4f", costVal)

		case "memory_count":
			if ds.GetMemoryCount != nil {
				count := ds.GetMemoryCount(config.TargetID) // TargetID = tenantID
				actual = strconv.Itoa(count)
			}

		case "custom":
			if ds.GetCustomValue != nil {
				actual, err = ds.GetCustomValue(config.TargetID)
				if err != nil {
					return false, err
				}
			}

		default:
			return false, fmt.Errorf("unknown check_type: %s", config.CheckType)
		}

		return compare(actual, config.Operator, config.Value)
	}
}

// compare performs the comparison based on operator.
// For numeric values, it attempts numeric comparison; falls back to string comparison.
func compare(actual, operator, expected string) (bool, error) {
	switch operator {
	case "eq":
		return actual == expected, nil

	case "neq":
		return actual != expected, nil

	case "contains":
		return strings.Contains(actual, expected), nil

	case "gt", "lt", "gte", "lte":
		// Try numeric comparison first
		aNum, aErr := strconv.ParseFloat(actual, 64)
		eNum, eErr := strconv.ParseFloat(expected, 64)

		if aErr == nil && eErr == nil {
			switch operator {
			case "gt":
				return aNum > eNum, nil
			case "lt":
				return aNum < eNum, nil
			case "gte":
				return aNum >= eNum, nil
			case "lte":
				return aNum <= eNum, nil
			}
		}

		// Fallback to string comparison
		switch operator {
		case "gt":
			return actual > expected, nil
		case "lt":
			return actual < expected, nil
		case "gte":
			return actual >= expected, nil
		case "lte":
			return actual <= expected, nil
		}
	}

	return false, fmt.Errorf("unknown operator: %s", operator)
}
