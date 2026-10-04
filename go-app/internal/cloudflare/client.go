package cloudflare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// baseURL is a variable so tests can point the client at a fake API.
var baseURL = "https://api.cloudflare.com/client/v4"

// The default http.Client has no timeout; a stalled Cloudflare call would
// hang the request that triggered it.
var httpClient = &http.Client{Timeout: 20 * time.Second}

type Client struct {
	apiToken string
}

type apiResponse struct {
	Success    bool            `json:"success"`
	Errors     []apiError      `json:"errors"`
	Result     json.RawMessage `json:"result"`
	ResultInfo *resultInfo     `json:"result_info"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type resultInfo struct {
	Page       int `json:"page"`
	TotalPages int `json:"total_pages"`
}

type DNSRecord struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Proxied  bool   `json:"proxied"`
	Priority *int   `json:"priority,omitempty"`
}

// Zone is a domain in the Cloudflare account.
type Zone struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Account struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"account"`
}

// TokenCheck reports what an API token can do, so the UI can say exactly
// which permission is missing instead of a bare "invalid token".
type TokenCheck struct {
	Valid           bool   `json:"valid"`
	CanListZones    bool   `json:"canListZones"`
	ZoneCount       int    `json:"zoneCount"`
	CanReadDNS      bool   `json:"canReadDns"`
	CanReadSettings bool   `json:"canReadSettings"`
	Error           string `json:"error,omitempty"`
}

func NewClient(apiToken string) *Client {
	return &Client{apiToken: apiToken}
}

// VerifyToken reports whether the token is usable. Cloudflare has two kinds:
// user tokens verify at /user/tokens/verify, account-owned tokens don't, so
// a token that can list zones is accepted as well.
func (c *Client) VerifyToken() bool {
	if _, _, err := c.request("GET", "/user/tokens/verify", nil); err == nil {
		return true
	}
	_, err := c.ListZones()
	return err == nil
}

// Check probes the read permissions DockLite uses. Edit permissions can't be
// tested without changing something, so the setup guide lists them instead.
func (c *Client) Check() TokenCheck {
	var check TokenCheck
	zones, err := c.ListZones()
	if err != nil {
		if _, _, verr := c.request("GET", "/user/tokens/verify", nil); verr == nil {
			check.Valid = true
			check.Error = "The token works but can't list your domains — add the Zone → Zone → Read permission."
		} else {
			check.Error = err.Error()
		}
		return check
	}
	check.Valid = true
	check.CanListZones = true
	check.ZoneCount = len(zones)
	if len(zones) == 0 {
		check.Error = "The token can't see any domains — check its Zone Resources include your domains."
		return check
	}
	if _, err := c.ListDNSRecords(zones[0].ID); err == nil {
		check.CanReadDNS = true
	}
	if _, err := c.GetZoneSetting(zones[0].ID, "ssl"); err == nil {
		check.CanReadSettings = true
	}
	return check
}

// ListZones returns every zone the token can see.
func (c *Client) ListZones() ([]Zone, error) {
	var all []Zone
	for page := 1; ; page++ {
		body, info, err := c.request("GET", fmt.Sprintf("/zones?per_page=50&page=%d", page), nil)
		if err != nil {
			return nil, err
		}
		var zones []Zone
		if err := json.Unmarshal(body, &zones); err != nil {
			return nil, err
		}
		all = append(all, zones...)
		if info == nil || page >= info.TotalPages {
			return all, nil
		}
	}
}

func (c *Client) ListDNSRecords(zoneID string) ([]DNSRecord, error) {
	var all []DNSRecord
	for page := 1; ; page++ {
		body, info, err := c.request("GET", fmt.Sprintf("/zones/%s/dns_records?per_page=100&page=%d", url.PathEscape(zoneID), page), nil)
		if err != nil {
			return nil, err
		}
		var records []DNSRecord
		if err := json.Unmarshal(body, &records); err != nil {
			return nil, err
		}
		all = append(all, records...)
		if info == nil || page >= info.TotalPages {
			return all, nil
		}
	}
}

func recordPayload(record DNSRecord) map[string]any {
	payload := map[string]any{
		"type":    record.Type,
		"name":    record.Name,
		"content": record.Content,
		"ttl":     record.TTL,
		"proxied": record.Proxied,
	}
	if record.Priority != nil {
		payload["priority"] = *record.Priority
	}
	return payload
}

func (c *Client) CreateDNSRecord(zoneID string, record DNSRecord) (*DNSRecord, error) {
	body, _, err := c.request("POST", fmt.Sprintf("/zones/%s/dns_records", url.PathEscape(zoneID)), recordPayload(record))
	if err != nil {
		return nil, err
	}
	var created DNSRecord
	if err := json.Unmarshal(body, &created); err != nil {
		return nil, err
	}
	return &created, nil
}

// UpdateDNSRecord replaces a record (PUT), so every field must be set.
func (c *Client) UpdateDNSRecord(zoneID string, recordID string, record DNSRecord) (*DNSRecord, error) {
	body, _, err := c.request("PUT", fmt.Sprintf("/zones/%s/dns_records/%s", url.PathEscape(zoneID), url.PathEscape(recordID)), recordPayload(record))
	if err != nil {
		return nil, err
	}
	var updated DNSRecord
	if err := json.Unmarshal(body, &updated); err != nil {
		return nil, err
	}
	return &updated, nil
}

func (c *Client) DeleteDNSRecord(zoneID string, recordID string) error {
	_, _, err := c.request("DELETE", fmt.Sprintf("/zones/%s/dns_records/%s", url.PathEscape(zoneID), url.PathEscape(recordID)), nil)
	return err
}

// GetZoneSetting reads one zone setting, e.g. "ssl" (off, flexible, full,
// strict) or "always_use_https" (on, off).
func (c *Client) GetZoneSetting(zoneID string, name string) (string, error) {
	body, _, err := c.request("GET", fmt.Sprintf("/zones/%s/settings/%s", url.PathEscape(zoneID), url.PathEscape(name)), nil)
	if err != nil {
		return "", err
	}
	var setting struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(body, &setting); err != nil {
		return "", err
	}
	return setting.Value, nil
}

func (c *Client) SetZoneSetting(zoneID string, name string, value string) error {
	_, _, err := c.request("PATCH", fmt.Sprintf("/zones/%s/settings/%s", url.PathEscape(zoneID), url.PathEscape(name)), map[string]any{"value": value})
	return err
}

func (c *Client) request(method string, endpoint string, payload any) (json.RawMessage, *resultInfo, error) {
	var bodyBytes []byte
	var err error
	if payload != nil {
		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, nil, err
		}
	}

	req, err := http.NewRequest(method, baseURL+endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var response apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, nil, fmt.Errorf("Cloudflare API error: unexpected response (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode/100 != 2 || !response.Success {
		message := "Cloudflare API error"
		if len(response.Errors) > 0 && response.Errors[0].Message != "" {
			message = response.Errors[0].Message
		}
		if resp.StatusCode == http.StatusForbidden {
			message += " (the token is missing a permission for this)"
		}
		return nil, nil, fmt.Errorf("Cloudflare API error: %s", message)
	}

	return response.Result, response.ResultInfo, nil
}
