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
	mu        sync.Mutex
	Accounts  []account `json:"accounts"`
	LastUsed  int       `json:"last_used"` // index in Accounts (0-based), -1 if none
	filePath  string
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
			return nil
		}
		return err
	}
	return json.Unmarshal(data, s)
}

func (s *accountsStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.LastUsed = len(s.Accounts) - 1
	return s.LastUsed + 1, nil
}

// UpdateToken обновляет access и refresh токен аккаунта по 1-based индексу. Сохраняет файл.
func (s *accountsStore) UpdateToken(accountIndex1 int, accessToken, refreshToken string) error {
	s.mu.Lock()
	if accountIndex1 < 1 || accountIndex1 > len(s.Accounts) {
		s.mu.Unlock()
		return os.ErrNotExist
	}
	s.Accounts[accountIndex1-1].Token = accessToken
	if refreshToken != "" {
		s.Accounts[accountIndex1-1].RefreshToken = refreshToken
	}
	s.mu.Unlock()
	return s.Save()
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

// List returns a copy of accounts for display (1-based indices).
func (s *accountsStore) List() []struct {
	Num   int
	Name  string
	Current bool
} {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]struct {
		Num   int
		Name  string
		Current bool
	}, len(s.Accounts))
	for i, a := range s.Accounts {
		out[i].Num = i + 1
		out[i].Name = a.displayName()
		out[i].Current = i == s.LastUsed
	}
	return out
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
	if accountIndex1 < 1 || accountIndex1 > len(s.Accounts) {
		s.mu.Unlock()
		return os.ErrNotExist
	}
	s.Accounts[accountIndex1-1].Name = strings.TrimSpace(name)
	s.mu.Unlock()
	err := s.Save()
	s.mu.Lock()
	return err
}

// SetProxy задаёт прокси аккаунту по 1-based индексу и сохраняет файл.
func (s *accountsStore) SetProxy(accountIndex1 int, proxy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if accountIndex1 < 1 || accountIndex1 > len(s.Accounts) {
		return os.ErrNotExist
	}
	s.Accounts[accountIndex1-1].Proxy = strings.TrimSpace(proxy)
	s.mu.Unlock()
	err := s.Save()
	s.mu.Lock()
	return err
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
