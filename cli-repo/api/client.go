package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

type HealthResponse struct {
	Status string `json:"status"`
}

type SummaryResponse struct {
	ContainersRunning int `json:"containers_running"`
	ContainersStopped int `json:"containers_stopped"`
	ImagesCount       int `json:"images_count"`
	VolumesCount      int `json:"volumes_count"`
}

type Container struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Status  string            `json:"status"`
	State   string            `json:"state"`
	Created int64             `json:"created"`
	Labels  map[string]string `json:"labels"`
	Ports   string            `json:"ports"`
	Uptime  string            `json:"uptime"`
	Owner   string            `json:"owner_username"`
}

// Group is a card that holds several boxes (see /api/groups). Members are container names, in order.
type Group struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

type ContainersResponse struct {
	Containers []Container `json:"containers"`
	Groups     []Group     `json:"groups"`
}

type ContainerStats struct {
	CPUUsage    float64 `json:"cpuUsage"`
	MemoryUsage uint64  `json:"memoryUsage"`
	MemoryLimit uint64  `json:"memoryLimit"`
	MemoryPct   float64 `json:"memoryPct"`
}

type ContainerStatsResponse struct {
	Stats *ContainerStats `json:"stats"`
}

type Database struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Status   string `json:"status"`
}

type DatabasesResponse struct {
	Databases []Database `json:"databases"`
}

type FileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

type FilesResponse struct {
	Path    string      `json:"path"`
	Entries []FileEntry `json:"entries"`
}

type FileContentResponse struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("request failed: %d - %s", e.Status, e.Body)
}

type CreateContainerRequest struct {
	Domain       string `json:"domain"`
	TemplateType string `json:"template_type"`
	IncludeWww   bool   `json:"include_www"`
	Port         int    `json:"port,omitempty"`
}

type CreateDatabaseRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Port     int    `json:"port,omitempty"`
}

type CreateDatabaseResponse struct {
	Database struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"database"`
	Connection struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Database string `json:"database"`
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"connection"`
}

type CreatePathRequest struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

type SaveFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type DeletePathRequest struct {
	Path string `json:"path"`
}

type RenamePathRequest struct {
	Path    string `json:"path"`
	NewName string `json:"new_name"`
}

func NewClient() *Client {
	baseURL := os.Getenv("DOCKLITE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:3000"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	token := os.Getenv("DOCKLITE_TOKEN")

	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *Client) SetBaseURL(baseURL string) {
	c.BaseURL = strings.TrimRight(baseURL, "/")
}

func (c *Client) SetToken(token string) {
	c.Token = token
}

func (c *Client) doRequest(method string, path string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequest(method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return c.HTTPClient.Do(req)
}

func (c *Client) GetHealth() (*HealthResponse, error) {
	resp, err := c.doRequest(http.MethodGet, "/api/health", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{Status: resp.StatusCode, Body: string(body)}
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, err
	}

	return &health, nil
}

func (c *Client) GetSummary() (*SummaryResponse, error) {
	resp, err := c.doRequest(http.MethodGet, "/api/summary", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{Status: resp.StatusCode, Body: string(body)}
	}

	var summary SummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return nil, err
	}

	return &summary, nil
}

// GetContainers asks for every box and the groups holding them. Agents older than groups answer /api/containers
// without a "boxes" list, so those fall back to the flat /api/containers/all (no groups).
func (c *Client) GetContainers() (*ContainersResponse, error) {
	resp, err := c.doRequest(http.MethodGet, "/api/containers", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{Status: resp.StatusCode, Body: string(body)}
	}
	var raw struct {
		Boxes  []Container `json:"boxes"`
		Groups []Group     `json:"groups"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err == nil && raw.Boxes != nil {
		return &ContainersResponse{Containers: raw.Boxes, Groups: raw.Groups}, nil
	}
	return c.getAllContainers()
}

func (c *Client) getAllContainers() (*ContainersResponse, error) {
	resp, err := c.doRequest(http.MethodGet, "/api/containers/all", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{Status: resp.StatusCode, Body: string(body)}
	}

	var containers ContainersResponse
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil, err
	}

	return &containers, nil
}

// UpdateStatus is /api/system/update/status: what is installed, what is newest, and how a running update is going.
type UpdateStatus struct {
	Version         string   `json:"version"`
	LatestVersion   string   `json:"latestVersion"`
	UpdateAvailable bool     `json:"updateAvailable"`
	CheckError      string   `json:"checkError"`
	UpdateRunning   bool     `json:"updateRunning"`
	State           string   `json:"state"`
	StateMessage    string   `json:"stateMessage"`
	Log             []string `json:"log"`
}

func (c *Client) GetUpdateStatus(refresh bool) (*UpdateStatus, error) {
	path := "/api/system/update/status"
	if refresh {
		path += "?refresh=1"
	}
	resp, err := c.doRequest(http.MethodGet, path, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{Status: resp.StatusCode, Body: string(body)}
	}
	var st UpdateStatus
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return nil, err
	}
	return &st, nil
}

// RunUpdate starts the same update the dashboard's Update button does: the agent, the web dashboard and this
// terminal app are all rebuilt from the newest release.
func (c *Client) RunUpdate() error {
	resp, err := c.doRequest(http.MethodPost, "/api/system/update/run", bytes.NewReader([]byte("{}")), "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{Status: resp.StatusCode, Body: string(body)}
	}
	return nil
}

func (c *Client) StartContainer(id string) error {
	resp, err := c.doRequest(http.MethodPost, "/api/containers/"+id+"/start", nil, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{Status: resp.StatusCode, Body: string(body)}
	}
	return nil
}

func (c *Client) StopContainer(id string) error {
	resp, err := c.doRequest(http.MethodPost, "/api/containers/"+id+"/stop", nil, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{Status: resp.StatusCode, Body: string(body)}
	}
	return nil
}

func (c *Client) RestartContainer(id string) error {
	resp, err := c.doRequest(http.MethodPost, "/api/containers/"+id+"/restart", nil, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{Status: resp.StatusCode, Body: string(body)}
	}
	return nil
}

func (c *Client) DeleteContainer(id string) error {
	resp, err := c.doRequest(http.MethodDelete, "/api/containers/"+id, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{Status: resp.StatusCode, Body: string(body)}
	}
	return nil
}

func (c *Client) GetContainerLogs(id string) (string, error) {
	resp, err := c.doRequest(http.MethodGet, "/api/containers/"+id+"/logs", nil, "")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", &APIError{Status: resp.StatusCode, Body: string(body)}
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result["logs"], nil
}

func (c *Client) GetContainerStats(id string) (*ContainerStats, error) {
	resp, err := c.doRequest(http.MethodGet, "/api/containers/"+id+"/stats", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{Status: resp.StatusCode, Body: string(body)}
	}

	var result ContainerStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Stats, nil
}

func (c *Client) CreateSiteContainer(domain string, templateType string, includeWww bool, port int) error {
	payload := CreateContainerRequest{
		Domain:       domain,
		TemplateType: templateType,
		IncludeWww:   includeWww,
		Port:         port,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := c.doRequest(http.MethodPost, "/api/containers", bytes.NewReader(body), "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return &APIError{Status: resp.StatusCode, Body: string(respBody)}
	}
	return nil
}

func (c *Client) GetDatabases() (*DatabasesResponse, error) {
	resp, err := c.doRequest(http.MethodGet, "/api/databases", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{Status: resp.StatusCode, Body: string(body)}
	}

	var databases DatabasesResponse
	if err := json.NewDecoder(resp.Body).Decode(&databases); err != nil {
		return nil, err
	}

	return &databases, nil
}

func (c *Client) CreateDatabase(name string, dbType string, username string, password string, port int) (*CreateDatabaseResponse, error) {
	payload := CreateDatabaseRequest{
		Name:     name,
		Type:     dbType,
		Username: username,
		Password: password,
		Port:     port,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(http.MethodPost, "/api/databases", bytes.NewReader(body), "application/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, &APIError{Status: resp.StatusCode, Body: string(respBody)}
	}

	var result CreateDatabaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) ListFiles(path string) (*FilesResponse, error) {
	query := ""
	if strings.TrimSpace(path) != "" {
		query = "?path=" + url.QueryEscape(path)
	}
	resp, err := c.doRequest(http.MethodGet, "/api/files"+query, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{Status: resp.StatusCode, Body: string(body)}
	}

	var result FilesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ReadFile(path string) (*FileContentResponse, error) {
	query := "?path=" + url.QueryEscape(path)
	resp, err := c.doRequest(http.MethodGet, "/api/files/content"+query, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{Status: resp.StatusCode, Body: string(body)}
	}

	var result FileContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) SaveFile(path string, content string) error {
	payload := SaveFileRequest{
		Path:    path,
		Content: content,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := c.doRequest(http.MethodPut, "/api/files/content", bytes.NewReader(body), "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return &APIError{Status: resp.StatusCode, Body: string(respBody)}
	}
	return nil
}

func (c *Client) CreatePath(path string, kind string) error {
	payload := CreatePathRequest{
		Path: path,
		Type: kind,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := c.doRequest(http.MethodPost, "/api/files/create", bytes.NewReader(body), "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return &APIError{Status: resp.StatusCode, Body: string(respBody)}
	}
	return nil
}

func (c *Client) DeletePath(path string) error {
	payload := DeletePathRequest{Path: path}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := c.doRequest(http.MethodPost, "/api/files/delete", bytes.NewReader(body), "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return &APIError{Status: resp.StatusCode, Body: string(respBody)}
	}
	return nil
}

func (c *Client) RenamePath(path string, newName string) error {
	payload := RenamePathRequest{
		Path:    path,
		NewName: newName,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := c.doRequest(http.MethodPost, "/api/files/rename", bytes.NewReader(body), "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return &APIError{Status: resp.StatusCode, Body: string(respBody)}
	}
	return nil
}
