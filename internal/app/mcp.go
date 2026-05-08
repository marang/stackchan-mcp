package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"stackchan-mcp/internal/issuework"
	"stackchan-mcp/internal/linearclient"
	"stackchan-mcp/internal/search"
	"strings"
	"sync"
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

var stdoutMu sync.Mutex

func runMCPServer() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}

		if req.Method == "tools/call" {
			go handle(req)
			continue
		}
		handle(req)
	}
}
func handle(req request) {
	switch req.Method {
	case "initialize":
		writeResult(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "stackchan-mcp",
				"version": "1.0.0",
			},
		})
	case "notifications/initialized":
		return
	case "ping":
		writeResult(req.ID, map[string]any{})
	case "tools/list":
		writeResult(req.ID, map[string]any{
			"tools": []map[string]any{
				{
					"name":        "say_hello",
					"description": "Returns a short greeting from Markus' local computer.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{
								"type":        "string",
								"description": "Name to greet",
							},
						},
					},
				},
					{
						"name":        "search_internet",
					"description": "Searches the web for a query and returns result titles, links, and snippets. If a URL is provided, searches that page and can optionally follow links from it.",
					"inputSchema": map[string]any{
						"type":     "object",
						"required": []string{"query"},
						"properties": map[string]any{
							"url": map[string]any{
								"type":        "string",
								"description": "Optional HTTP or HTTPS URL to open and search within",
							},
							"query": map[string]any{
								"type":        "string",
								"description": "Search query or term to search for on the page",
							},
							"max_results": map[string]any{
								"type":        "number",
								"description": "Maximum number of results to return, from 1 to 10",
							},
							"follow_links": map[string]any{
								"type":        "boolean",
								"description": "When url is provided, also search pages linked from that page",
							},
							"max_pages": map[string]any{
								"type":        "number",
								"description": "Maximum pages to fetch when following links, from 1 to 12",
							},
							"same_host_only": map[string]any{
								"type":        "boolean",
								"description": "Only follow links on the same host as the starting URL, default true",
							},
						},
					},
				},
					{
						"name":        "start_ticket_work",
					"description": "Voice-friendly shortcut to start one Linear ticket by team key and number, e.g. RIOT 123. It fetches the issue from Linear using the API key stored in Secret Service.",
					"inputSchema": map[string]any{
						"type":     "object",
						"required": []string{"team", "number"},
						"properties": map[string]any{
							"team": map[string]any{
								"type":        "string",
								"description": "Linear team key, e.g. RIOT",
							},
							"number": map[string]any{
								"type":        "number",
								"description": "Linear ticket number, e.g. 123",
							},
							"repo": map[string]any{
								"type":        "string",
								"description": "Optional repo name or path; if omitted, the team key is mapped to a repo",
							},
							"dry_run": map[string]any{
								"type":        "boolean",
								"description": "Validate and return planned session without changing git or tmux",
							},
							"start_implementation": map[string]any{
								"type":        "boolean",
								"description": "Send an implementation prompt to the Codex tmux pane after preparing the session, default true",
							},
							"implementation_prompt": map[string]any{
								"type":        "string",
								"description": "Optional custom prompt to send to Codex",
							},
						},
					},
				},
				{
					"name":        "linear_list_teams",
					"description": "Lists Linear teams using the Linear API key stored in Secret Service.",
					"inputSchema": map[string]any{
						"type":       "object",
						"properties": map[string]any{},
					},
				},
				{
					"name":        "resolve_project",
					"description": "Finds git repositories under ~/Dev by project name or validates a project path.",
					"inputSchema": map[string]any{
						"type":     "object",
						"required": []string{"query"},
						"properties": map[string]any{
							"query": map[string]any{
								"type":        "string",
								"description": "Project name such as riotbox, or a path such as ~/Dev/riotbox",
							},
						},
					},
				},
				{
					"name":        "finish_issue",
					"description": "Records a completion message for an issue and returns a speakable notification string.",
					"inputSchema": map[string]any{
						"type":     "object",
						"required": []string{"issue_key"},
						"properties": map[string]any{
							"issue_key": map[string]any{
								"type":        "string",
								"description": "Linear issue key, e.g. RIOT-123",
							},
							"message": map[string]any{
								"type":        "string",
								"description": "Completion message",
							},
							"worktree_path": map[string]any{
								"type":        "string",
								"description": "Optional worktree path for reports/CONVO_FEED.log",
							},
						},
					},
				},
			},
		})
	case "tools/call":
		handleToolCall(req)
	default:
		if req.ID != nil {
			writeError(req.ID, -32601, fmt.Sprintf("Unknown method: %s", req.Method))
		}
	}
}

func handleToolCall(req request) {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeError(req.ID, -32602, "Invalid tool call parameters")
		return
	}

	switch params.Name {
	case "say_hello":
		name, _ := params.Arguments["name"].(string)
		if name == "" {
			name = "StackChan"
		}
		writeText(req.ID, fmt.Sprintf("Hello %s, this comes from Markus' Go MCP.", name))
	case "search_internet":
		result, err := search.SearchInternet(params.Arguments)
		if err != nil {
			writeError(req.ID, -32603, err.Error())
			return
		}
		writeText(req.ID, result)
	case "start_ticket_work":
		result, err := startTicketWork(params.Arguments)
		if err != nil {
			writeError(req.ID, -32603, err.Error())
			return
		}
		writeText(req.ID, result)
	case "linear_list_teams":
		result, err := linearListTeams()
		if err != nil {
			writeError(req.ID, -32603, err.Error())
			return
		}
		writeText(req.ID, result)
	case "resolve_project":
		result, err := resolveProject(params.Arguments)
		if err != nil {
			writeError(req.ID, -32603, err.Error())
			return
		}
		writeText(req.ID, result)
	case "finish_issue":
		result, err := finishIssue(params.Arguments)
		if err != nil {
			writeError(req.ID, -32603, err.Error())
			return
		}
		writeText(req.ID, result)
	default:
		writeError(req.ID, -32602, "Unknown tool: "+params.Name)
	}
}

func startTicketWork(args map[string]any) (string, error) {
	team, _ := args["team"].(string)
	team = strings.ToUpper(strings.TrimSpace(team))
	if team == "" {
		return "", fmt.Errorf("team is required")
	}

	number := numberArg(args, "number", 0)
	if number <= 0 {
		return "", fmt.Errorf("number is required")
	}

	repo, _ := args["repo"].(string)
	repo = strings.TrimSpace(repo)
	if repo == "" {
		repo = defaultRepoForTeam(team)
	}

	resolved, err := issuework.ResolveProject(repo)
	if err != nil {
		return "", err
	}
	if len(resolved.Candidates) != 1 {
		data, _ := json.MarshalIndent(resolved, "", "  ")
		return "", fmt.Errorf("team %s maps to an ambiguous repo; candidates: %s", team, string(data))
	}

	key := fmt.Sprintf("%s-%d", team, number)
	linearIssue, err := getLinearIssue(key)
	if err != nil {
		return "", err
	}
	issue := issuework.Issue{
		Key:         linearIssue.Identifier,
		Number:      linearIssue.Number,
		Title:       linearIssue.Title,
		URL:         linearIssue.URL,
		BranchName:  linearIssue.BranchName,
		Description: linearIssue.Description,
	}

	manifest := issuework.Manifest{
		ProjectPath: resolved.Candidates[0].Path,
		RepoName:    resolved.Candidates[0].Name,
		Issues:      []issuework.Issue{issue},
	}

	startImplementation := true
	if _, ok := args["start_implementation"]; ok {
		startImplementation = boolArg(args, "start_implementation", true)
	}
	prompt, _ := args["implementation_prompt"].(string)

	dryRun := boolArg(args, "dry_run", false)
	result, err := issuework.Start(manifest, issuework.StartOptions{DryRun: dryRun})
	if err != nil {
		return "", err
	}
	if len(result.Sessions) == 0 {
		return "", fmt.Errorf("no session was created")
	}

	session := result.Sessions[0]
	source := "with Linear data"
	promptStatus := "Implementation prompt was not requested."
	if startImplementation && !dryRun {
		promptStatus = "Implementation prompt was queued for Codex."
		go func() {
			if err := issuework.PromptForIssue(session.SessionName, session.WorktreePath, manifest.RepoName, issue, prompt); err != nil {
				_ = issuework.AppendPromptError(session.WorktreePath, issue.Key, session.SessionName, err)
			}
		}()
	} else if dryRun {
		promptStatus = "Dry run only; implementation prompt was not sent."
	}
	return fmt.Sprintf("Prepared %s %s. Title: %s. Worktree: %s. tmux session: %s. %s Attach with: %s", issue.Key, source, issue.Title, session.WorktreePath, session.SessionName, promptStatus, session.AttachCommand), nil
}

func getLinearIssue(identifier string) (linearclient.Issue, error) {
	client, err := linearclient.NewFromSecretStore()
	if err != nil {
		return linearclient.Issue{}, err
	}
	return client.GetIssue(identifier)
}

func linearListTeams() (string, error) {
	client, err := linearclient.NewFromSecretStore()
	if err != nil {
		return "", err
	}
	teams, err := client.ListTeams()
	if err != nil {
		return "", err
	}
	if len(teams) == 0 {
		return "No Linear teams found.", nil
	}
	var b strings.Builder
	b.WriteString("Linear Teams:\n")
	for _, team := range teams {
		fmt.Fprintf(&b, "- %s: %s\n", team.Key, team.Name)
	}
	return strings.TrimSpace(b.String()), nil
}

func fallbackStringArg(args map[string]any, name string, fallback string) string {
	value, _ := args[name].(string)
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func defaultRepoForTeam(team string) string {
	switch strings.ToUpper(strings.TrimSpace(team)) {
	case "RIOT":
		return "riotbox"
	default:
		return strings.ToLower(team)
	}
}

func resolveProject(args map[string]any) (string, error) {
	query, _ := args["query"].(string)
	result, err := issuework.ResolveProject(query)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func finishIssue(args map[string]any) (string, error) {
	issueKey, _ := args["issue_key"].(string)
	message, _ := args["message"].(string)
	worktreePath, _ := args["worktree_path"].(string)
	result, err := issuework.Finish(issueKey, message, worktreePath)
	if err != nil {
		return "", err
	}
	if result.LogPath != "" {
		return fmt.Sprintf("%s Log: %s", result.Message, result.LogPath), nil
	}
	return result.Message, nil
}
func writeText(id any, text string) {
	writeResult(id, map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": text,
			},
		},
	})
}

func writeResult(id any, result any) {
	writeJSON(response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func writeError(id any, code int, message string) {
	writeJSON(response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
		},
	})
}

func writeJSON(v any) {
	encoded, err := json.Marshal(v)
	if err != nil {
		return
	}
	stdoutMu.Lock()
	defer stdoutMu.Unlock()
	fmt.Println(string(encoded))
}
