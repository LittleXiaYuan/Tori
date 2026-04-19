package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	RepoPath    string            `json:"repo_path"`
	RepoURL     string            `json:"repo_url,omitempty"`
	Description string            `json:"description,omitempty"`
	DefaultCaps []string          `json:"default_caps,omitempty"`
	Meta        map[string]string `json:"meta,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type ProjectStore struct {
	mu       sync.RWMutex
	projects map[string]*Project
	baseDir  string
}

func NewProjectStore(dir string) *ProjectStore {
	s := &ProjectStore{
		projects: make(map[string]*Project),
		baseDir:  dir,
	}
	s.loadAll()
	return s
}

type CreateProjectRequest struct {
	Name        string            `json:"name"`
	RepoPath    string            `json:"repo_path"`
	RepoURL     string            `json:"repo_url,omitempty"`
	Description string            `json:"description,omitempty"`
	DefaultCaps []string          `json:"default_caps,omitempty"`
	Meta        map[string]string `json:"meta,omitempty"`
}

func (r *CreateProjectRequest) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("project name is required")
	}
	if r.RepoPath == "" {
		return fmt.Errorf("repo_path is required")
	}
	return nil
}

func (s *ProjectStore) Create(req CreateProjectRequest) (*Project, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	now := time.Now()
	p := &Project{
		ID:          uuid.New().String()[:8],
		Name:        req.Name,
		RepoPath:    req.RepoPath,
		RepoURL:     req.RepoURL,
		Description: req.Description,
		DefaultCaps: req.DefaultCaps,
		Meta:        req.Meta,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.mu.Lock()
	s.projects[p.ID] = p
	s.mu.Unlock()

	if err := s.save(p); err != nil {
		return nil, fmt.Errorf("persist project: %w", err)
	}
	return p, nil
}

func (s *ProjectStore) Get(id string) (*Project, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.projects[id]
	if !ok {
		return nil, false
	}
	cp := *p
	return &cp, true
}

func (s *ProjectStore) FindByName(name string) (*Project, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.projects {
		if p.Name == name {
			cp := *p
			return &cp, true
		}
	}
	return nil, false
}

func (s *ProjectStore) List() []*Project {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Project, 0, len(s.projects))
	for _, p := range s.projects {
		cp := *p
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func (s *ProjectStore) Update(p *Project) error {
	p.UpdatedAt = time.Now()
	s.mu.Lock()
	s.projects[p.ID] = p
	err := s.save(p)
	s.mu.Unlock()
	return err
}

func (s *ProjectStore) Delete(id string) bool {
	s.mu.Lock()
	_, ok := s.projects[id]
	if ok {
		delete(s.projects, id)
	}
	s.mu.Unlock()
	if ok {
		os.Remove(filepath.Join(s.baseDir, id+".json"))
	}
	return ok
}

func (s *ProjectStore) save(p *Project) error {
	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.baseDir, p.ID+".json"), data, 0o644)
}

func (s *ProjectStore) loadAll() {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.baseDir, e.Name()))
		if err != nil {
			continue
		}
		var p Project
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		s.projects[p.ID] = &p
	}
}
