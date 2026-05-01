package instruction

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type UserInstruction struct {
	ID        string    `json:"instruction_id"`
	TenantID  string    `json:"tenant_id"`
	Category  string    `json:"category"`
	Content   string    `json:"content"`
	Priority  int       `json:"priority"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type KV interface {
	Get(ctx context.Context, key string, dest any) (bool, error)
	Put(ctx context.Context, key string, value any) error
}

type Store struct {
	kv KV
}

func NewStore(kv KV) *Store {
	return &Store{kv: kv}
}

func kvKey(tenantID string) string { return tenantID + ":all" }

func (s *Store) List(ctx context.Context, tenantID string) ([]UserInstruction, error) {
	var list []UserInstruction
	found, err := s.kv.Get(ctx, kvKey(tenantID), &list)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Priority > list[j].Priority })
	return list, nil
}

func (s *Store) Create(ctx context.Context, tenantID string, inst UserInstruction) (*UserInstruction, error) {
	list, err := s.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(inst.Content)
	if content == "" {
		return nil, fmt.Errorf("instruction content cannot be empty")
	}
	for _, existing := range list {
		if existing.Content == content && existing.Category == inst.Category {
			return &existing, nil
		}
	}
	now := time.Now()
	inst.ID = uuid.New().String()
	inst.TenantID = tenantID
	inst.Content = content
	inst.IsActive = true
	inst.CreatedAt = now
	inst.UpdatedAt = now
	if inst.Priority == 0 {
		inst.Priority = 50
	}
	list = append(list, inst)
	if err := s.kv.Put(ctx, kvKey(tenantID), list); err != nil {
		return nil, err
	}
	return &inst, nil
}

func (s *Store) Update(ctx context.Context, tenantID string, inst UserInstruction) error {
	list, err := s.List(ctx, tenantID)
	if err != nil {
		return err
	}
	found := false
	for i, existing := range list {
		if existing.ID == inst.ID {
			if inst.Content != "" {
				list[i].Content = strings.TrimSpace(inst.Content)
			}
			if inst.Category != "" {
				list[i].Category = inst.Category
			}
			if inst.Priority > 0 {
				list[i].Priority = inst.Priority
			}
			list[i].IsActive = inst.IsActive
			list[i].UpdatedAt = time.Now()
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("instruction %s not found", inst.ID)
	}
	return s.kv.Put(ctx, kvKey(tenantID), list)
}

func (s *Store) Delete(ctx context.Context, tenantID, instructionID string) error {
	list, err := s.List(ctx, tenantID)
	if err != nil {
		return err
	}
	filtered := list[:0]
	for _, inst := range list {
		if inst.ID != instructionID {
			filtered = append(filtered, inst)
		}
	}
	if len(filtered) == len(list) {
		return fmt.Errorf("instruction %s not found", instructionID)
	}
	return s.kv.Put(ctx, kvKey(tenantID), filtered)
}

func (s *Store) Reorder(ctx context.Context, tenantID string, ids []string) error {
	list, err := s.List(ctx, tenantID)
	if err != nil {
		return err
	}
	idxMap := make(map[string]int, len(ids))
	for i, id := range ids {
		idxMap[id] = i
	}
	base := 100
	for i := range list {
		if pos, ok := idxMap[list[i].ID]; ok {
			list[i].Priority = base - pos
			list[i].UpdatedAt = time.Now()
		}
	}
	return s.kv.Put(ctx, kvKey(tenantID), list)
}

// ActiveForTenant returns all active instructions for prompt injection, sorted by priority descending.
func (s *Store) ActiveForTenant(ctx context.Context, tenantID string) ([]UserInstruction, error) {
	list, err := s.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	var active []UserInstruction
	for _, inst := range list {
		if inst.IsActive {
			active = append(active, inst)
		}
	}
	return active, nil
}
