package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Game struct {
	Serial     string `json:"serial"`
	RedumpHash string `json:"redump_hash"`
	Title      string `json:"title"`
	SizeBytes  int64  `json:"size_bytes"`
}

type Manifest struct {
	Version     int        `json:"version"`
	Serial      string     `json:"serial"`
	RedumpHash  string     `json:"redump_hash"`
	Title       string     `json:"title"`
	SizeBytes   int64      `json:"size_bytes"`
	BlockSize   int        `json:"block_size"`
	BootRanges  [][2]int64 `json:"boot_ranges"`
	FileURL     string     `json:"file_url"`
	ManifestURL string     `json:"manifest_url"`
	Supported   bool       `json:"supported"`
}

type Client struct {
	base  string
	http  *http.Client
	token string
}

func New(base, token string) *Client {
	return &Client{base: base, http: &http.Client{}, token: token}
}

func (c *Client) get(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.base+path, nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", path, res.Status)
	}
	return res, nil
}

func ListGames(c *Client) ([]Game, error) {
	res, err := c.get("/v1/games")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var out struct {
		Games []Game `json:"games"`
	}
	return out.Games, json.NewDecoder(res.Body).Decode(&out)
}

func GetManifest(c *Client, serial string) (Manifest, error) {
	var m Manifest
	res, err := c.get("/v1/games/" + url.PathEscape(serial) + "/manifest.json")
	if err != nil {
		return m, err
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&m); err != nil {
		return m, err
	}
	if m.Version != 2 {
		return m, fmt.Errorf("manifest v%d unsupported, update server and client together", m.Version)
	}
	return m, nil
}

func (c *Client) DownloadRange(serial string, offset, length int64, w io.Writer) error {
	req, err := http.NewRequest("GET", c.base+"/v1/files/"+url.PathEscape(serial), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+length-1))
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("range %d+%d: %s", offset, length, res.Status)
	}
	_, err = io.Copy(w, res.Body)
	return err
}

func Health(base string) error {
	res, err := http.Get(base + "/v1/health")
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("health: %s", res.Status)
	}
	return nil
}

func Login(base, username, password string) (string, error) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(map[string]string{"username": username, "password": password}); err != nil {
		return "", err
	}
	res, err := http.Post(base+"/v1/auth/login", "application/json", &body)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login: %s", res.Status)
	}
	var out struct {
		Token string `json:"token"`
	}
	return out.Token, json.NewDecoder(res.Body).Decode(&out)
}

type SaveMeta struct {
	SHA     string
	Size    int64
	Updated time.Time
}

// GetSave fetches a memory card. Returns os.ErrNotExist wrapped when the
// server has never seen one (HTTP 404).
func (c *Client) GetSave(serial string, slot int) ([]byte, SaveMeta, error) {
	var meta SaveMeta
	res, err := c.get("/v1/saves/" + url.PathEscape(serial) + "/" + strconv.Itoa(slot))
	if err != nil {
		return nil, meta, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 16<<20+1))
	if err != nil {
		return nil, meta, err
	}
	meta.SHA = res.Header.Get("X-SHA256")
	if t, err := time.Parse(http.TimeFormat, res.Header.Get("Last-Modified")); err == nil {
		meta.Updated = t
	}
	meta.Size = int64(len(raw))
	return raw, meta, nil
}

// HeadSave returns server metadata without downloading bytes.
// ok=false with nil error means the server has no save (HTTP 404).
func (c *Client) HeadSave(serial string, slot int) (meta SaveMeta, ok bool, err error) {
	req, err := http.NewRequest("HEAD",
		c.base+"/v1/saves/"+url.PathEscape(serial)+"/"+strconv.Itoa(slot), nil)
	if err != nil {
		return meta, false, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return meta, false, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return meta, false, nil
	}
	if res.StatusCode != http.StatusOK {
		return meta, false, fmt.Errorf("HEAD save: %s", res.Status)
	}
	meta.SHA = res.Header.Get("X-SHA256")
	if t, err := time.Parse(http.TimeFormat, res.Header.Get("Last-Modified")); err == nil {
		meta.Updated = t
	}
	return meta, true, nil
}

// PutSave uploads a memory card, returning the stored metadata.
func (c *Client) PutSave(serial string, slot int, data []byte) (SaveMeta, error) {
	var meta SaveMeta
	req, err := http.NewRequest("PUT",
		c.base+"/v1/saves/"+url.PathEscape(serial)+"/"+strconv.Itoa(slot),
		bytes.NewReader(data))
	if err != nil {
		return meta, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return meta, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return meta, fmt.Errorf("PUT save: %s", res.Status)
	}
	var out struct {
		SHA256    string `json:"sha256"`
		Size      int64  `json:"size"`
		UpdatedAt string `json:"updated_at"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return meta, err
	}
	meta.SHA = out.SHA256
	meta.Size = out.Size
	if t, err := time.Parse(time.RFC3339, out.UpdatedAt); err == nil {
		meta.Updated = t
	}
	return meta, nil
}
