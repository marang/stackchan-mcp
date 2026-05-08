package app

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"stackchan-mcp/internal/issuework"
	"stackchan-mcp/internal/linearclient"
	"stackchan-mcp/internal/secretstore"
	"strings"
)

func xiaoZhiStoreURL(args []string) {
	fs := flag.NewFlagSet("xiaozhi-store-url", flag.ExitOnError)
	rawURL := fs.String("url", "", "XiaoZhi MCP WebSocket URL; if omitted, read from stdin")
	_ = fs.Parse(args)

	value := strings.TrimSpace(*rawURL)
	if value == "" {
		value = prompt("Paste XiaoZhi MCP WebSocket URL: ")
	}
	if err := storeXiaoZhiURL(value); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Stored XiaoZhi MCP URL in Secret Service as service=stackchan-mcp account=xiaozhi-mcp-url")
}

func linearStoreAPIKey(args []string) {
	fs := flag.NewFlagSet("linear-store-api-key", flag.ExitOnError)
	apiKey := fs.String("key", "", "Linear API key; if omitted, read from stdin")
	_ = fs.Parse(args)

	key := strings.TrimSpace(*apiKey)
	if key == "" {
		key = prompt("Paste Linear API key: ")
	}

	if err := linearclient.SaveAPIKey(key); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Stored Linear API key in Secret Service as service=stackchan-mcp account=linear-api-key")
}

func setup(args []string) {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	force := fs.Bool("force", false, "prompt for secrets even when they are already stored")
	_ = fs.Parse(args)

	if *force || !hasSecret("service", "stackchan-mcp", "account", "xiaozhi-mcp-url") {
		value := prompt("Paste XiaoZhi MCP WebSocket URL: ")
		if err := storeXiaoZhiURL(value); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Stored XiaoZhi MCP URL in Secret Service as service=stackchan-mcp account=xiaozhi-mcp-url")
	} else {
		fmt.Println("XiaoZhi MCP URL is already stored in Secret Service.")
	}

	if *force || !hasSecret("service", "stackchan-mcp", "account", "linear-api-key") {
		key := prompt("Paste Linear API key: ")
		if err := linearclient.SaveAPIKey(key); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Stored Linear API key in Secret Service as service=stackchan-mcp account=linear-api-key")
	} else {
		fmt.Println("Linear API key is already stored in Secret Service.")
	}
}

func prompt(label string) string {
	fmt.Fprint(os.Stderr, label)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}
	return strings.TrimSpace(line)
}

func hasSecret(attrs ...string) bool {
	_, err := secretstore.Lookup(attrs...)
	return err == nil
}

func storeXiaoZhiURL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("XiaoZhi MCP URL is empty")
	}
	return secretstore.Store("StackChan MCP XiaoZhi URL", value, "service", "stackchan-mcp", "account", "xiaozhi-mcp-url")
}

func resolve(args []string) {
	fs := flag.NewFlagSet("resolve", flag.ExitOnError)
	project := fs.String("project", "", "project name or path")
	_ = fs.Parse(args)

	result, err := issuework.ResolveProject(*project)
	if err != nil {
		log.Fatal(err)
	}
	printPrettyJSON(result)
}

func startIssueWorkCommand(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	manifestPath := fs.String("manifest", "", "path to issue-work manifest JSON")
	dryRun := fs.Bool("dry-run", false, "validate and print planned sessions without changing git or tmux")
	_ = fs.Parse(args)

	if *manifestPath == "" {
		log.Fatal("missing --manifest")
	}

	manifest, err := issuework.LoadManifest(*manifestPath)
	if err != nil {
		log.Fatal(err)
	}

	result, err := issuework.Start(manifest, issuework.StartOptions{DryRun: *dryRun})
	if err != nil {
		log.Fatal(err)
	}
	printPrettyJSON(result)
	for _, session := range result.Sessions {
		fmt.Fprintf(os.Stderr, "Attach with: %s\n", session.AttachCommand)
	}
}

func finishIssueWork(args []string) {
	fs := flag.NewFlagSet("finish", flag.ExitOnError)
	issueKey := fs.String("issue", "", "Linear issue key, e.g. RIOT-123")
	message := fs.String("message", "", "completion message")
	worktreePath := fs.String("worktree", "", "optional worktree path for reports/CONVO_FEED.log")
	_ = fs.Parse(args)

	result, err := issuework.Finish(*issueKey, *message, *worktreePath)
	if err != nil {
		log.Fatal(err)
	}
	printPrettyJSON(result)
}

func printPrettyJSON(v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(data))
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  stackchan-mcp bridge [--debug] [--ws url]")
	fmt.Fprintln(os.Stderr, "  stackchan-mcp serve")
	fmt.Fprintln(os.Stderr, "  stackchan-mcp setup [--force]")
	fmt.Fprintln(os.Stderr, "  stackchan-mcp xiaozhi-store-url")
	fmt.Fprintln(os.Stderr, "  stackchan-mcp linear-store-api-key")
	fmt.Fprintln(os.Stderr, "  stackchan-mcp resolve --project riotbox")
	fmt.Fprintln(os.Stderr, "  stackchan-mcp start --manifest manifest.json [--dry-run]")
	fmt.Fprintln(os.Stderr, "  stackchan-mcp finish --issue RIOT-123 [--message text] [--worktree path]")
}
