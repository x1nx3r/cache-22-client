package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/x1nx3r/cache-22-client/internal/api"
)

type ServerEntry struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

type serverFile struct {
	Servers []ServerEntry `json:"servers"`
	Active  string        `json:"active"`
}

type ServerService struct {
	path string
}

func NewServerService(dataDir string) *ServerService {
	return &ServerService{path: filepath.Join(dataDir, "servers.json")}
}

func (s *ServerService) load() (serverFile, error) {
	var f serverFile
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return f, nil
		}
		return f, err
	}
	return f, json.Unmarshal(raw, &f)
}

func (s *ServerService) save(f serverFile) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o600)
}

func (s *ServerService) Servers() ([]ServerEntry, error) {
	f, err := s.load()
	if err != nil {
		return nil, err
	}
	if f.Servers == nil {
		return []ServerEntry{}, nil
	}
	return f.Servers, nil
}

func (s *ServerService) Active() (string, error) {
	f, err := s.load()
	if err != nil {
		return "", err
	}
	return f.Active, nil
}

func (s *ServerService) SetActive(url string) error {
	f, err := s.load()
	if err != nil {
		return err
	}
	f.Active = normalizeURL(url)
	return s.save(f)
}

func (s *ServerService) Entry(url string) (ServerEntry, error) {
	f, err := s.load()
	if err != nil {
		return ServerEntry{}, err
	}
	for _, e := range f.Servers {
		if e.URL == normalizeURL(url) {
			return e, nil
		}
	}
	return ServerEntry{}, fmt.Errorf("server not registered")
}

func (s *ServerService) ActiveEntry() (ServerEntry, error) {
	active, err := s.Active()
	if err != nil {
		return ServerEntry{}, err
	}
	if active == "" {
		return ServerEntry{}, errNoServer
	}
	return s.Entry(active)
}

func (s *ServerService) AddServer(name, url, username, password string) error {
	name = strings.TrimSpace(name)
	url = normalizeURL(url)
	if name == "" || url == "" {
		return fmt.Errorf("name and url are required")
	}
	if username == "" || password == "" {
		return fmt.Errorf("username and password are required, the server admin must provision your account")
	}
	token, err := api.Login(url, username, password)
	if err != nil {
		if strings.Contains(err.Error(), "401") {
			return fmt.Errorf("invalid username or password")
		}
		return err
	}
	f, err := s.load()
	if err != nil {
		return err
	}
	for _, e := range f.Servers {
		if e.URL == url {
			return fmt.Errorf("server already registered")
		}
	}
	f.Servers = append(f.Servers, ServerEntry{Name: name, URL: url, Username: username, Token: token})
	if f.Active == "" {
		f.Active = url
	}
	return s.save(f)
}

func (s *ServerService) RemoveServer(url string) error {
	url = normalizeURL(url)
	f, err := s.load()
	if err != nil {
		return err
	}
	kept := f.Servers[:0]
	for _, e := range f.Servers {
		if e.URL != url {
			kept = append(kept, e)
		}
	}
	f.Servers = kept
	if f.Active == url {
		f.Active = ""
		if len(kept) > 0 {
			f.Active = kept[0].URL
		}
	}
	return s.save(f)
}

func (s *ServerService) TestConnection(url, username, password string) error {
	url = normalizeURL(url)
	if username != "" || password != "" {
		if _, err := api.Login(url, username, password); err != nil {
			if strings.Contains(err.Error(), "401") {
				return fmt.Errorf("invalid username or password")
			}
			return err
		}
		return nil
	}
	return api.Health(url)
}

func normalizeURL(url string) string {
	url = strings.TrimRight(strings.TrimSpace(url), "/")
	if url != "" && !strings.Contains(url, "://") {
		url = "http://" + url
	}
	return url
}
