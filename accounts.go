package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"sync"
)

const accountsFile = ".kick_accounts.json"

type account struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Proxy        string `json:"proxy,omitempty"` // SOCKS5: host:port:user:pass
}

type accountsStore struct {
	mu           sync.Mutex
	Accounts     []account `json:"accounts"`
	LastUsed     int       `json:"last_used"`               // index in Accounts (0-based), -1 if none
	DisplayOrder []int     `json:"display_order,omitempty"` // stable account IDs, display only
	filePath     string
}

type accountListItem struct {
	Num      int
	StableID int
	Name     string
	Current  bool
}

func newAccountsStore(path string) *accountsStore {
	if path == "" {
		path = accountsFile
	}
	return &accountsStore{filePath: path, LastUsed: -1}
}

func (s *accountsStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.Accounts = nil
			s.LastUsed = -1
			s.DisplayOrder = nil
			return nil
		}
		return err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return err
	}
	s.normalizeDisplayOrderLocked()
	return nil
}

func (s *accountsStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *accountsStore) saveLocked() error {
	s.normalizeDisplayOrderLocked()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0600)
}

func (s *accountsStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Accounts)
}

// Current returns (token, name, proxy, true) for the current account, or ("", "", "", false) if none.
func (s *accountsStore) Current() (token, name, proxy string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.LastUsed < 0 || s.LastUsed >= len(s.Accounts) {
		return "", "", "", false
	}
	a := s.Accounts[s.LastUsed]
	return a.Token, a.displayName(), a.Proxy, true
}

func (a account) displayName() string {
	if a.Name != "" {
		return a.Name
	}
	return "Account " + strconv.Itoa(a.ID)
}

// Add adds a new account and sets it as current. refreshToken may be empty (старые аккаунты без refresh).
func (s *accountsStore) Add(name, token, refreshToken string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := 1
	for _, a := range s.Accounts {
		if a.ID >= id {
			id = a.ID + 1
		}
	}
	s.Accounts = append(s.Accounts, account{ID: id, Name: name, Token: token, RefreshToken: refreshToken, Proxy: ""})
	s.DisplayOrder = append(s.DisplayOrder, id)
	s.LastUsed = len(s.Accounts) - 1
	return s.LastUsed + 1, nil
}

// UpdateToken обновляет access и refresh токен аккаунта по 1-based индексу. Сохраняет файл.
func (s *accountsStore) UpdateToken(accountIndex1 int, accessToken, refreshToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if accountIndex1 < 1 || accountIndex1 > len(s.Accounts) {
		return os.ErrNotExist
	}
	s.Accounts[accountIndex1-1].Token = accessToken
	if refreshToken != "" {
		s.Accounts[accountIndex1-1].RefreshToken = refreshToken
	}
	return s.saveLocked()
}

// Use sets current account by 1-based index. Returns true if found.
func (s *accountsStore) Use(index1 int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index1 < 1 || index1 > len(s.Accounts) {
		return false
	}
	s.LastUsed = index1 - 1
	return true
}

// GetAccountByIndex returns (token, refreshToken, proxy, name, true) for 1-based index, or ("", "", "", "", false).
func (s *accountsStore) GetAccountByIndex(i int) (token, refreshToken, proxy, name string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i < 1 || i > len(s.Accounts) {
		return "", "", "", "", false
	}
	a := s.Accounts[i-1]
	return a.Token, a.RefreshToken, a.Proxy, a.displayName(), true
}

// SetCurrent sets the current account by 1-based index (for dashboard).
func (s *accountsStore) SetCurrent(index1 int) bool {
	return s.Use(index1)
}

func (s *accountsStore) normalizeDisplayOrderLocked() {
	valid := make(map[int]bool, len(s.Accounts))
	for _, a := range s.Accounts {
		valid[a.ID] = true
	}
	seen := make(map[int]bool, len(s.Accounts))
	order := make([]int, 0, len(s.Accounts))
	for _, id := range s.DisplayOrder {
		if valid[id] && !seen[id] {
			seen[id] = true
			order = append(order, id)
		}
	}
	for _, a := range s.Accounts {
		if !seen[a.ID] {
			seen[a.ID] = true
			order = append(order, a.ID)
		}
	}
	s.DisplayOrder = order
}

// List returns accounts in persisted display order while Num remains the operational 1-based index.
func (s *accountsStore) List() []accountListItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.normalizeDisplayOrderLocked()
	byStableID := make(map[int]int, len(s.Accounts))
	for i, a := range s.Accounts {
		byStableID[a.ID] = i
	}
	out := make([]accountListItem, 0, len(s.Accounts))
	for _, stableID := range s.DisplayOrder {
		i := byStableID[stableID]
		a := s.Accounts[i]
		out = append(out, accountListItem{
			Num:      i + 1,
			StableID: a.ID,
			Name:     a.displayName(),
			Current:  i == s.LastUsed,
		})
	}
	return out
}

// PositionByStableID resolves a persisted account ID to the operational 1-based index.
func (s *accountsStore) PositionByStableID(stableID int) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, a := range s.Accounts {
		if a.ID == stableID {
			return i + 1, true
		}
	}
	return 0, false
}

// SetDisplayOrder validates and persists a complete permutation of stable account IDs.
func (s *accountsStore) SetDisplayOrder(order []int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(order) != len(s.Accounts) {
		return os.ErrInvalid
	}
	valid := make(map[int]bool, len(s.Accounts))
	for _, a := range s.Accounts {
		valid[a.ID] = true
	}
	seen := make(map[int]bool, len(order))
	for _, id := range order {
		if !valid[id] || seen[id] {
			return os.ErrInvalid
		}
		seen[id] = true
	}
	s.DisplayOrder = append([]int(nil), order...)
	return s.saveLocked()
}

// NameAt returns the custom label for account index (1-based), or empty if unset.
func (s *accountsStore) NameAt(accountIndex1 int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if accountIndex1 < 1 || accountIndex1 > len(s.Accounts) {
		return ""
	}
	return s.Accounts[accountIndex1-1].Name
}

// SetName sets the display label for an account (1-based index).
func (s *accountsStore) SetName(accountIndex1 int, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if accountIndex1 < 1 || accountIndex1 > len(s.Accounts) {
		return os.ErrNotExist
	}
	s.Accounts[accountIndex1-1].Name = strings.TrimSpace(name)
	return s.saveLocked()
}

// SetProxy задаёт прокси аккаунту по 1-based индексу и сохраняет файл.
func (s *accountsStore) SetProxy(accountIndex1 int, proxy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if accountIndex1 < 1 || accountIndex1 > len(s.Accounts) {
		return os.ErrNotExist
	}
	s.Accounts[accountIndex1-1].Proxy = strings.TrimSpace(proxy)
	return s.saveLocked()
}

// assignProxiesFromLines sets Account.Proxy from lines: line i → account i. Only sets if account.Proxy is empty.
func (s *accountsStore) assignProxiesFromLines(lines []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || i >= len(s.Accounts) {
			continue
		}
		if s.Accounts[i].Proxy == "" {
			s.Accounts[i].Proxy = line
		}
	}
}
