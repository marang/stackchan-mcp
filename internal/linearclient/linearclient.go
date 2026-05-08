package linearclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"stackchan-mcp/internal/secretstore"
	"strings"
	"time"
)

const endpoint = "https://api.linear.app/graphql"

type Client struct {
	apiKey string
	http   http.Client
}

type Team struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type Issue struct {
	ID          string `json:"id"`
	Identifier  string `json:"identifier"`
	Number      int    `json:"number"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	BranchName  string `json:"branchName"`
	Team        Team   `json:"team"`
}

func New(apiKey string) (*Client, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("Linear API key is required")
	}
	return &Client{
		apiKey: apiKey,
		http: http.Client{
			Timeout: 20 * time.Second,
		},
	}, nil
}

func NewFromSecretStore() (*Client, error) {
	apiKey, err := LoadAPIKey()
	if err != nil && !errors.Is(err, secretstore.ErrNotFound) {
		return nil, err
	}
	if apiKey != "" {
		return New(apiKey)
	}
	return nil, errors.New("Linear API key is not stored; run stackchan-mcp linear-store-api-key")
}

func LoadAPIKey() (string, error) {
	return secretstore.Lookup("service", "stackchan-mcp", "account", "linear-api-key")
}

func SaveAPIKey(apiKey string) error {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return errors.New("Linear API key is empty")
	}
	return secretstore.Store("StackChan MCP Linear API Key", apiKey, "service", "stackchan-mcp", "account", "linear-api-key")
}

func (c *Client) ListTeams() ([]Team, error) {
	var resp struct {
		Teams struct {
			Nodes []Team `json:"nodes"`
		} `json:"teams"`
	}
	err := c.graphQL(`query Teams { teams { nodes { id key name } } }`, nil, &resp)
	return resp.Teams.Nodes, err
}

func (c *Client) GetIssue(identifier string) (Issue, error) {
	var resp struct {
		Issue Issue `json:"issue"`
	}
	err := c.graphQL(`query Issue($id: String!) {
  issue(id: $id) {
    id
    identifier
    number
    title
    url
    description
    branchName
    team { id key name }
  }
}`, map[string]any{"id": strings.TrimSpace(identifier)}, &resp)
	if err != nil {
		return Issue{}, err
	}
	if resp.Issue.ID == "" {
		return Issue{}, fmt.Errorf("Linear issue not found: %s", identifier)
	}
	return resp.Issue, nil
}

func (c *Client) graphQL(query string, variables map[string]any, target any) error {
	payload, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": variables,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var decoded struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Linear returned HTTP %s", resp.Status)
	}
	if len(decoded.Errors) > 0 {
		return fmt.Errorf("Linear error: %s", decoded.Errors[0].Message)
	}
	if len(decoded.Data) == 0 || string(decoded.Data) == "null" {
		return errors.New("Linear returned no data")
	}
	return json.Unmarshal(decoded.Data, target)
}
