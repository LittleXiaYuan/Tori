package cogni

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// FederationPeer represents a remote Yunque Agent instance.
type FederationPeer struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	URL      string    `json:"url"`
	APIKey   string    `json:"api_key,omitempty"`
	LastSeen time.Time `json:"last_seen"`
	Status   string    `json:"status"` // online | offline | unknown
	Cognis   []string  `json:"cognis,omitempty"`
}

// FederatedSkill is a skill exposed by a remote cogni via federation.
type FederatedSkill struct {
	CogniID     string `json:"cogni_id"`
	SkillName   string `json:"skill_name"`
	Description string `json:"description"`
	PeerID      string `json:"peer_id"`
	PeerURL     string `json:"peer_url"`
}

// CogniFederation manages cross-instance cogni sharing.
type CogniFederation struct {
	mu       sync.RWMutex
	selfID   string
	selfURL  string
	peers    map[string]*FederationPeer
	exposed  map[string]*Declaration // cognis exposed to federation
	registry *Registry
	client   *http.Client
}

func NewCogniFederation(selfID, selfURL string, registry *Registry) *CogniFederation {
	return &CogniFederation{
		selfID:   selfID,
		selfURL:  selfURL,
		peers:    make(map[string]*FederationPeer),
		exposed:  make(map[string]*Declaration),
		registry: registry,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// AddPeer registers a remote agent instance.
func (cf *CogniFederation) AddPeer(peer FederationPeer) {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	peer.Status = "unknown"
	cf.peers[peer.ID] = &peer
	slog.Info("federation: peer added", "id", peer.ID, "url", peer.URL)
}

// RemovePeer unregisters a remote agent instance.
func (cf *CogniFederation) RemovePeer(id string) {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	delete(cf.peers, id)
}

// Peers returns all registered peers.
func (cf *CogniFederation) Peers() []FederationPeer {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	out := make([]FederationPeer, 0, len(cf.peers))
	for _, p := range cf.peers {
		out = append(out, *p)
	}
	return out
}

// Expose marks a cogni's skills as available for remote invocation.
func (cf *CogniFederation) Expose(cogniID string) error {
	decl, ok := cf.registry.Get(cogniID)
	if !ok {
		return fmt.Errorf("federation: cogni %q not found", cogniID)
	}
	cf.mu.Lock()
	cf.exposed[cogniID] = decl
	cf.mu.Unlock()
	slog.Info("federation: cogni exposed", "id", cogniID)
	return nil
}

// Unexpose removes a cogni from federation exposure.
func (cf *CogniFederation) Unexpose(cogniID string) {
	cf.mu.Lock()
	delete(cf.exposed, cogniID)
	cf.mu.Unlock()
}

// ExposedCognis returns all cognis available for remote invocation.
func (cf *CogniFederation) ExposedCognis() []*Declaration {
	cf.mu.RLock()
	defer cf.mu.RUnlock()
	out := make([]*Declaration, 0, len(cf.exposed))
	for _, d := range cf.exposed {
		out = append(out, d)
	}
	return out
}

// DiscoverRemoteSkills queries all peers for their exposed cogni skills.
func (cf *CogniFederation) DiscoverRemoteSkills(ctx context.Context) []FederatedSkill {
	cf.mu.RLock()
	peers := make([]*FederationPeer, 0, len(cf.peers))
	for _, p := range cf.peers {
		peers = append(peers, p)
	}
	cf.mu.RUnlock()

	var mu sync.Mutex
	var skills []FederatedSkill

	var wg sync.WaitGroup
	for _, peer := range peers {
		wg.Add(1)
		go func(p *FederationPeer) {
			defer wg.Done()
			remote, err := cf.fetchPeerCognis(ctx, p)
			if err != nil {
				cf.mu.Lock()
				p.Status = "offline"
				cf.mu.Unlock()
				return
			}
			cf.mu.Lock()
			p.Status = "online"
			p.LastSeen = time.Now()
			p.Cognis = nil
			cf.mu.Unlock()

			mu.Lock()
			for _, d := range remote {
				cf.mu.Lock()
				p.Cognis = append(p.Cognis, d.ID)
				cf.mu.Unlock()
				for _, s := range d.Skills() {
					skills = append(skills, FederatedSkill{
						CogniID:     d.ID,
						SkillName:   s,
						PeerID:      p.ID,
						PeerURL:     p.URL,
					})
				}
			}
			mu.Unlock()
		}(peer)
	}
	wg.Wait()
	return skills
}

// fetchPeerCognis queries a peer's /v1/cognis endpoint.
func (cf *CogniFederation) fetchPeerCognis(ctx context.Context, peer *FederationPeer) ([]*federatedDecl, error) {
	url := peer.URL + "/v1/cognis"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if peer.APIKey != "" {
		req.Header.Set("X-API-Key", peer.APIKey)
	}
	resp, err := cf.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("peer %s returned %d", peer.ID, resp.StatusCode)
	}

	var body struct {
		Cognis []federatedDecl `json:"cognis"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	out := make([]*federatedDecl, len(body.Cognis))
	for i := range body.Cognis {
		out[i] = &body.Cognis[i]
	}
	return out, nil
}

// federatedDecl is compatible with both the remote /v1/cognis response
// (EntryStatus: id, display_name, description) and any future explicit
// skill_names field.
type federatedDecl struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	SkillNames  []string `json:"skill_names,omitempty"`
}

func (d *federatedDecl) Skills() []string {
	if len(d.SkillNames) > 0 {
		return d.SkillNames
	}
	if d.ID != "" {
		return []string{d.ID}
	}
	return nil
}

// Stats returns federation statistics.
func (cf *CogniFederation) Stats() map[string]any {
	cf.mu.RLock()
	defer cf.mu.RUnlock()

	online := 0
	for _, p := range cf.peers {
		if p.Status == "online" {
			online++
		}
	}
	return map[string]any{
		"self_id":       cf.selfID,
		"peers_total":   len(cf.peers),
		"peers_online":  online,
		"exposed_cognis": len(cf.exposed),
	}
}
