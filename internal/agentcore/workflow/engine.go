package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"yunque-agent/internal/observe"
	"yunque-agent/pkg/skills"
)

// ──────────────────────────────────────────────
// Engine — DAG workflow execution engine
//
// Execution model:
//  1. Topologically sort nodes
//  2. Execute ready nodes (all predecessors complete)
//  3. Parallel nodes run concurrently
//  4. Condition nodes evaluate and pick a branch
//  5. Join nodes wait for all incoming edges
//  6. Engine checkpoints after each node completion
// ──────────────────────────────────────────────

// SkillExecutor calls a skill by name with the given args.
type SkillExecutor func(ctx context.Context, skillName string, args map[string]any) (string, error)

// LLMExecutor calls the LLM with system+user prompts.
type LLMExecutor func(ctx context.Context, system, user string) (string, error)

// BrowserExecutor runs a browser action (navigate/click/type/screenshot/read/eval).
type BrowserExecutor func(ctx context.Context, action string, args map[string]any) (string, error)

// CodeExecutor runs sandboxed code and returns output.
type CodeExecutor func(ctx context.Context, language, code string) (string, error)

// KnowledgeExecutor queries the knowledge base.
type KnowledgeExecutor func(ctx context.Context, query string, topK int) (string, error)

// Engine executes workflow DAGs.
type Engine struct {
	store    Store
	skills   *skills.Registry
	execSkill   SkillExecutor
	execLLM     LLMExecutor
	execBrowser BrowserExecutor
	execCode    CodeExecutor
	execKnow    KnowledgeExecutor

	mu       sync.Mutex
	running  map[string]context.CancelFunc // instanceID → cancel

	eventMu    sync.RWMutex
	listeners  []WorkflowEventListener
}

// WorkflowEvent describes a workflow execution event.
type WorkflowEvent struct {
	Type       string `json:"type"`        // e.g. "node_start", "node_done", "node_failed", "workflow_done"
	InstanceID string `json:"instance_id"`
	NodeID     string `json:"node_id,omitempty"`
	NodeName   string `json:"node_name,omitempty"`
	NodeType   string `json:"node_type,omitempty"`
	Message    string `json:"message"`
	Error      string `json:"error,omitempty"`
}

// WorkflowEventListener is called for workflow execution events.
type WorkflowEventListener func(event observe.AgentEvent)

// NewEngine creates a workflow execution engine.
func NewEngine(store Store, registry *skills.Registry, execSkill SkillExecutor, execLLM LLMExecutor) *Engine {
	return &Engine{
		store:     store,
		skills:    registry,
		execSkill: execSkill,
		execLLM:   execLLM,
		running:   make(map[string]context.CancelFunc),
	}
}

// SetBrowserExecutor sets the browser automation executor.
func (e *Engine) SetBrowserExecutor(fn BrowserExecutor) { e.execBrowser = fn }

// SetCodeExecutor sets the sandboxed code executor.
func (e *Engine) SetCodeExecutor(fn CodeExecutor) { e.execCode = fn }

// SetKnowledgeExecutor sets the knowledge base query executor.
func (e *Engine) SetKnowledgeExecutor(fn KnowledgeExecutor) { e.execKnow = fn }

// OnEvent registers a listener for workflow execution events.
func (e *Engine) OnEvent(fn WorkflowEventListener) {
	e.eventMu.Lock()
	defer e.eventMu.Unlock()
	e.listeners = append(e.listeners, fn)
}

// emit fires an event to all listeners, converting to unified AgentEvent.
func (e *Engine) emit(evt WorkflowEvent) {
	e.eventMu.RLock()
	defer e.eventMu.RUnlock()
	if len(e.listeners) == 0 {
		return
	}
	agentEvt := observe.NewEvent("", observe.DomainWorkflow, evt.Type, evt.Message)
	agentEvt.Meta.InstanceID = evt.InstanceID
	agentEvt.Meta.NodeID = evt.NodeID
	agentEvt.Meta.NodeName = evt.NodeName
	agentEvt.Detail = evt
	for _, fn := range e.listeners {
		go fn(agentEvt)
	}
}

// Run starts executing a workflow instance. Blocks until completion.
func (e *Engine) Run(ctx context.Context, instanceID string) error {
	inst, err := e.store.GetInstance(instanceID)
	if err != nil {
		return fmt.Errorf("get instance: %w", err)
	}
	def, err := e.store.GetDefinition(inst.DefinitionID)
	if err != nil {
		return fmt.Errorf("get definition: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	e.mu.Lock()
	e.running[instanceID] = cancel
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.running, instanceID)
		e.mu.Unlock()
	}()

	// Build adjacency graph
	graph := buildGraph(def)

	// Initialize node states
	if inst.NodeStates == nil {
		inst.NodeStates = make(map[string]*NodeState)
	}
	for _, node := range def.Nodes {
		if _, exists := inst.NodeStates[node.ID]; !exists {
			inst.NodeStates[node.ID] = &NodeState{
				NodeID: node.ID,
				Status: NodePending,
			}
		}
	}

	// Mark instance as running
	now := time.Now()
	inst.Status = InstanceRunning
	inst.StartedAt = &now
	inst.UpdatedAt = now
	e.store.SaveInstance(inst)

	slog.Info("workflow: started", "instance", instanceID, "definition", def.Name)

	// Execute until all nodes done or error
	for {
		if ctx.Err() != nil {
			inst.Status = InstanceCancelled
			inst.Error = "cancelled"
			fin := time.Now()
			inst.FinishedAt = &fin
			inst.UpdatedAt = fin
			e.store.SaveInstance(inst)
			return ctx.Err()
		}

		readyNodes, skippedNodes := e.findReadyNodes(def, inst, graph)
		if len(skippedNodes) > 0 {
			for _, sid := range skippedNodes {
				ns := inst.NodeStates[sid]
				ns.Status = NodeSkipped
				now := time.Now()
				ns.FinishedAt = &now
				
				n := findNode(def, sid)
				e.emit(WorkflowEvent{
					Type: "node_skipped", InstanceID: inst.ID,
					NodeID: sid, NodeName: n.Name, NodeType: string(n.Type),
					Message: fmt.Sprintf("节点 [%s] 路径未命中，已跳过", n.Name),
				})
			}
			inst.UpdatedAt = time.Now()
			e.store.SaveInstance(inst)
			continue
		}

		if len(readyNodes) == 0 {
			// Check if all done or stuck
			if e.allNodesDone(inst) {
				break
			}
			// Deadlock or waiting for input
			hasWaiting := false
			for _, ns := range inst.NodeStates {
				if ns.Status == NodeWaiting {
					hasWaiting = true
					break
				}
			}
			if hasWaiting {
				inst.Status = InstancePaused
				inst.UpdatedAt = time.Now()
				e.store.SaveInstance(inst)
				return nil // will be resumed when input arrives
			}
			// Deadlock
			inst.Status = InstanceFailed
			inst.Error = "workflow deadlock: no ready nodes and not all complete"
			fin := time.Now()
			inst.FinishedAt = &fin
			inst.UpdatedAt = fin
			e.store.SaveInstance(inst)
			return fmt.Errorf("workflow deadlock")
		}

		// Execute ready nodes (concurrently if multiple)
		if len(readyNodes) == 1 {
			if err := e.executeNode(ctx, def, inst, readyNodes[0]); err != nil {
				inst.Status = InstanceFailed
				inst.Error = err.Error()
				fin := time.Now()
				inst.FinishedAt = &fin
				inst.UpdatedAt = fin
				e.store.SaveInstance(inst)
				return err
			}
		} else {
			// Parallel execution
			var wg sync.WaitGroup
			errCh := make(chan error, len(readyNodes))
			for _, nodeID := range readyNodes {
				nid := nodeID
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := e.executeNode(ctx, def, inst, nid); err != nil {
						errCh <- err
					}
				}()
			}
			wg.Wait()
			close(errCh)

			if firstErr, ok := <-errCh; ok {
				inst.Status = InstanceFailed
				inst.Error = firstErr.Error()
				fin := time.Now()
				inst.FinishedAt = &fin
				inst.UpdatedAt = fin
				e.store.SaveInstance(inst)
				return firstErr
			}
		}

		inst.UpdatedAt = time.Now()
		e.store.SaveInstance(inst)
	}

	// Complete
	inst.Status = InstanceCompleted
	fin := time.Now()
	inst.FinishedAt = &fin
	inst.UpdatedAt = fin
	e.store.SaveInstance(inst)
	slog.Info("workflow: completed", "instance", instanceID)
	return nil
}

// Cancel stops a running workflow instance.
func (e *Engine) Cancel(instanceID string) bool {
	e.mu.Lock()
	cancel, ok := e.running[instanceID]
	e.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

// executeNode runs a single node and updates its state.
func (e *Engine) executeNode(ctx context.Context, def *Definition, inst *Instance, nodeID string) error {
	node := findNode(def, nodeID)
	if node == nil {
		return fmt.Errorf("node %s not found in definition", nodeID)
	}

	ns := inst.NodeStates[nodeID]
	now := time.Now()
	ns.Status = NodeRunning
	ns.StartedAt = &now

	slog.Info("workflow: executing node", "instance", inst.ID, "node", nodeID, "type", node.Type)

	// Emit node_start event
	e.emit(WorkflowEvent{
		Type: "node_start", InstanceID: inst.ID,
		NodeID: nodeID, NodeName: node.Name, NodeType: string(node.Type),
		Message: fmt.Sprintf("节点 [%s] 开始执行", node.Name),
	})

	var result any
	var err error

	switch node.Type {
	case NodeSkill:
		result, err = e.execSkillNode(ctx, node, inst)
	case NodeLLM:
		result, err = e.execLLMNode(ctx, node, inst)
	case NodeCondition:
		result, err = e.execConditionNode(node, inst)
	case NodeTransform:
		result, err = e.execTransformNode(node, inst)
	case NodeInput:
		// Mark as waiting — will be resumed externally
		ns.Status = NodeWaiting
		return nil
	case NodeParallel, NodeJoin:
		// Structural nodes — auto-complete
		result = "ok"
	case NodeSubflow:
		result, err = e.execSubflowNode(ctx, node, inst)
	case NodeBrowser:
		result, err = e.execBrowserNode(ctx, node, inst)
	case NodeCode:
		result, err = e.execCodeNode(ctx, node, inst)
	case NodeKnowledge:
		result, err = e.execKnowledgeNode(ctx, node, inst)
	default:
		err = fmt.Errorf("unknown node type: %s", node.Type)
	}

	fin := time.Now()
	ns.FinishedAt = &fin

	if err != nil {
		ns.Status = NodeFailed
		ns.Error = err.Error()
		slog.Warn("workflow: node failed", "instance", inst.ID, "node", nodeID, "err", err)
		e.emit(WorkflowEvent{
			Type: "node_failed", InstanceID: inst.ID,
			NodeID: nodeID, NodeName: node.Name, NodeType: string(node.Type),
			Message: fmt.Sprintf("节点 [%s] 执行失败", node.Name),
			Error: err.Error(),
		})
		return fmt.Errorf("node %s (%s): %w", nodeID, node.Name, err)
	}

	ns.Status = NodeDone
	ns.Output = result
	e.emit(WorkflowEvent{
		Type: "node_done", InstanceID: inst.ID,
		NodeID: nodeID, NodeName: node.Name, NodeType: string(node.Type),
		Message: fmt.Sprintf("节点 [%s] 执行完成", node.Name),
	})

	// Store output in instance variables for downstream nodes
	if inst.Variables == nil {
		inst.Variables = make(map[string]any)
	}
	inst.Variables["_node_"+nodeID] = result

	return nil
}

// ── Node executors ──

func (e *Engine) execSkillNode(ctx context.Context, node *Node, inst *Instance) (any, error) {
	skillName, _ := node.Config["skill_name"].(string)
	if skillName == "" {
		return nil, fmt.Errorf("skill_name not specified")
	}
	args := make(map[string]any)
	if a, ok := node.Config["args"].(map[string]any); ok {
		args = a
	}
	// Resolve variable references in args
	for k, v := range args {
		if s, ok := v.(string); ok && len(s) > 2 && s[0] == '{' && s[len(s)-1] == '}' {
			varName := s[1 : len(s)-1]
			if val, exists := inst.Variables[varName]; exists {
				args[k] = val
			}
		}
	}
	result, err := e.execSkill(ctx, skillName, args)
	return result, err
}

func (e *Engine) execLLMNode(ctx context.Context, node *Node, inst *Instance) (any, error) {
	system, _ := node.Config["system_prompt"].(string)
	user, _ := node.Config["user_prompt"].(string)
	if user == "" {
		return nil, fmt.Errorf("user_prompt not specified")
	}
	// Simple variable substitution in prompts
	for k, v := range inst.Variables {
		placeholder := "{" + k + "}"
		if s, ok := v.(string); ok {
			system = replaceAll(system, placeholder, s)
			user = replaceAll(user, placeholder, s)
		}
	}
	return e.execLLM(ctx, system, user)
}

func (e *Engine) execConditionNode(node *Node, inst *Instance) (any, error) {
	// Simple condition: check variable truthiness
	varName, _ := node.Config["variable"].(string)
	if varName == "" {
		return "true", nil
	}
	val, exists := inst.Variables[varName]
	if !exists {
		return "false", nil
	}
	switch v := val.(type) {
	case bool:
		if v {
			return "true", nil
		}
		return "false", nil
	case string:
		if v != "" && v != "false" && v != "0" {
			return "true", nil
		}
		return "false", nil
	default:
		return "true", nil
	}
}

func (e *Engine) execTransformNode(node *Node, inst *Instance) (any, error) {
	// Simple key mapping transform
	template, _ := node.Config["template"].(string)
	if template == "" {
		return nil, fmt.Errorf("template not specified")
	}
	result := template
	for k, v := range inst.Variables {
		if s, ok := v.(string); ok {
			result = replaceAll(result, "{"+k+"}", s)
		}
	}
	return result, nil
}

func (e *Engine) execSubflowNode(ctx context.Context, node *Node, inst *Instance) (any, error) {
	subflowID, _ := node.Config["definition_id"].(string)
	if subflowID == "" {
		return nil, fmt.Errorf("definition_id not specified")
	}
	subInst, err := e.store.CreateInstance(subflowID, inst.TenantID, inst.Variables)
	if err != nil {
		return nil, fmt.Errorf("create subflow instance: %w", err)
	}
	if err := e.Run(ctx, subInst.ID); err != nil {
		return nil, fmt.Errorf("subflow failed: %w", err)
	}
	// Return subflow's final variables
	subInst, _ = e.store.GetInstance(subInst.ID)
	return subInst.Variables, nil
}

func (e *Engine) execBrowserNode(ctx context.Context, node *Node, inst *Instance) (any, error) {
	if e.execBrowser == nil {
		return nil, fmt.Errorf("browser executor not configured")
	}
	action, _ := node.Config["action"].(string)
	if action == "" {
		action = "navigate"
	}
	args := make(map[string]any)
	if target, ok := node.Config["target"].(string); ok {
		// Resolve variable references
		for k, v := range inst.Variables {
			if s, ok := v.(string); ok {
				target = replaceAll(target, "{"+k+"}", s)
			}
		}
		args["target"] = target
	}
	if text, ok := node.Config["text"].(string); ok {
		for k, v := range inst.Variables {
			if s, ok := v.(string); ok {
				text = replaceAll(text, "{"+k+"}", s)
			}
		}
		args["text"] = text
	}
	return e.execBrowser(ctx, action, args)
}

func (e *Engine) execCodeNode(ctx context.Context, node *Node, inst *Instance) (any, error) {
	if e.execCode == nil {
		return nil, fmt.Errorf("code executor not configured")
	}
	language, _ := node.Config["language"].(string)
	if language == "" {
		language = "javascript"
	}
	code, _ := node.Config["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code not specified")
	}
	// Resolve variable references in code
	for k, v := range inst.Variables {
		if s, ok := v.(string); ok {
			code = replaceAll(code, "{"+k+"}", s)
		}
	}
	return e.execCode(ctx, language, code)
}

func (e *Engine) execKnowledgeNode(ctx context.Context, node *Node, inst *Instance) (any, error) {
	if e.execKnow == nil {
		return nil, fmt.Errorf("knowledge executor not configured")
	}
	query, _ := node.Config["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query not specified")
	}
	topK := 5
	if k, ok := node.Config["top_k"].(float64); ok {
		topK = int(k)
	}
	// Resolve variable references in query
	for k, v := range inst.Variables {
		if s, ok := v.(string); ok {
			query = replaceAll(query, "{"+k+"}", s)
		}
	}
	return e.execKnow(ctx, query, topK)
}

// ── Graph helpers ──

type adjacency struct {
	predecessors map[string][]string // nodeID → list of predecessor nodeIDs
	successors   map[string][]string // nodeID → list of successor nodeIDs
}

func buildGraph(def *Definition) *adjacency {
	g := &adjacency{
		predecessors: make(map[string][]string),
		successors:   make(map[string][]string),
	}
	for _, node := range def.Nodes {
		if _, ok := g.predecessors[node.ID]; !ok {
			g.predecessors[node.ID] = nil
		}
		if _, ok := g.successors[node.ID]; !ok {
			g.successors[node.ID] = nil
		}
	}
	for _, edge := range def.Edges {
		g.predecessors[edge.ToNode] = append(g.predecessors[edge.ToNode], edge.FromNode)
		g.successors[edge.FromNode] = append(g.successors[edge.FromNode], edge.ToNode)
	}
	return g
}

func (e *Engine) findReadyNodes(def *Definition, inst *Instance, graph *adjacency) ([]string, []string) {
	var ready []string
	var skipped []string
	for _, node := range def.Nodes {
		ns := inst.NodeStates[node.ID]
		if ns.Status != NodePending {
			continue
		}
		
		allDone := true
		shouldSkip := false

		// Check incoming edges
		for _, edge := range def.Edges {
			if edge.ToNode == node.ID {
				predState := inst.NodeStates[edge.FromNode]
				if predState == nil || (predState.Status != NodeDone && predState.Status != NodeSkipped) {
					allDone = false
					break
				}
				if predState.Status == NodeSkipped {
					shouldSkip = true
				} else if edge.Condition != "" {
					outStr := fmt.Sprintf("%v", predState.Output)
					if outStr != edge.Condition {
						shouldSkip = true
					}
				}
			}
		}

		if allDone {
			if shouldSkip {
				skipped = append(skipped, node.ID)
			} else {
				ready = append(ready, node.ID)
			}
		}
	}
	return ready, skipped
}

func (e *Engine) allNodesDone(inst *Instance) bool {
	for _, ns := range inst.NodeStates {
		if ns.Status != NodeDone && ns.Status != NodeSkipped && ns.Status != NodeFailed {
			return false
		}
	}
	return true
}

func findNode(def *Definition, nodeID string) *Node {
	for i := range def.Nodes {
		if def.Nodes[i].ID == nodeID {
			return &def.Nodes[i]
		}
	}
	return nil
}

func replaceAll(s, old, new string) string {
	if old == "" {
		return s
	}
	for {
		i := indexOf(s, old)
		if i < 0 {
			return s
		}
		s = s[:i] + new + s[i+len(old):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
