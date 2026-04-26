/** LoRA scheduler pipeline state (JSON from Go localbrain.SchedulerState) */
export interface ABTestMetrics {
  new_adapter_queries: number;
  new_adapter_score: number;
  old_adapter_queries: number;
  old_adapter_score: number;
}

export interface TrainResult {
  adapter_name: string;
  adapter_path: string;
  samples: number;
  epochs: number;
  final_loss: number;
  /** Nanoseconds */
  duration: number;
  success: boolean;
  error?: string;
}

export interface SchedulerState {
  last_train_time: string;
  last_train_result?: TrainResult | null;
  current_adapter: string;
  previous_adapter: string;
  ab_test_active: boolean;
  ab_test_start: string;
  ab_test_metrics: ABTestMetrics;
  total_trains: number;
  total_rollbacks: number;
}

/** GET /v1/lora/status */
export interface LoRAStatus {
  scheduler: SchedulerState;
  active_model: string;
  rolling_success_rate?: number;
}

export interface FilterStats {
  total_lines?: number;
  kept_lines?: number;
  dropped_lines?: number;
}

/** Single training run (localbrain.TrainingRecord) */
export interface TrainingRecord {
  id: string;
  tenant_id: string;
  adapter_name: string;
  base_model: string;
  start_time: string;
  end_time: string;
  /** Nanoseconds */
  duration: number;
  samples: number;
  epochs: number;
  lora_rank: number;
  learning_rate: number;
  final_loss: number;
  success: boolean;
  error?: string;
  incremental: boolean;
  resume_from?: string;
  filter_stats?: FilterStats;
  eval_score?: number;
  eval_passed?: boolean;
  deployed: boolean;
  rolled_back: boolean;
}

/** Aggregate metrics (localbrain.TrainingSummary) */
export interface TrainingSummary {
  total_runs: number;
  success_count: number;
  failure_count: number;
  deploy_count: number;
  rollback_count: number;
  avg_loss: number;
  /** Nanoseconds */
  avg_duration: number;
  avg_samples: number;
  last_train_time: string;
  by_tenant: Record<string, number>;
}

/** LoRA scheduler configuration */
export interface LoRAConfig {
  min_samples: number;
  min_interval: number; // nanoseconds
  eval_min_score: number;
  max_adapters: number;
  base_model: string;
  training_data_dir: string;
  adapter_dir: string;
  ab_test_duration: number; // nanoseconds
  filter_enabled: boolean;
}

/** Evolution coordinator snapshot (localbrain.CoordinatorState) */
export interface EvolutionState {
  total_tasks: number;
  success_tasks: number;
  tasks_since_strategy: number;
  tasks_since_weights: number;
  last_strategy_update: string;
  last_weight_trigger: string;
  strategy_updates: number;
  weight_triggers: number;
  rolling_success_rate: number;
  recent_window: boolean[];
  by_tenant: Record<string, number>;
}
