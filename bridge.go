package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"stackchan-mcp/internal/secretstore"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

func runBridge(args []string) {
	fs := flag.NewFlagSet("bridge", flag.ExitOnError)
	wsURL := fs.String("ws", "", "full XiaoZhi MCP WebSocket URL from the Android app")
	mcpCommand := fs.String("mcp-command", "", "local MCP server command; defaults to this binary")
	mcpArgs := fs.String("mcp-args", "serve", "local MCP server args, separated by spaces")
	debug := fs.Bool("debug", false, "log JSON-RPC traffic without the WebSocket token")
	reconnect := fs.Bool("reconnect", true, "automatically reconnect when the WebSocket closes")
	_ = fs.Parse(args)

	storedURL, err := loadStoredXiaoZhiURL()
	if err != nil {
		log.Fatal(err)
	}

	endpoint, err := xiaoZhiEndpoint(firstNonEmpty(*wsURL, storedURL))
	if err != nil {
		log.Fatal(err)
	}

	command := firstNonEmpty(*mcpCommand, defaultSelfCommand())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runBridgeWithReconnect(ctx, endpoint, command, splitArgs(*mcpArgs), *debug, *reconnect); err != nil {
		log.Fatal(err)
	}
}

func defaultSelfCommand() string {
	exe, err := os.Executable()
	if err != nil {
		return "stackchan-mcp"
	}
	return exe
}

func loadStoredXiaoZhiURL() (string, error) {
	value, err := secretstore.Lookup("service", "stackchan-mcp", "account", "xiaozhi-mcp-url")
	if err == nil {
		return value, nil
	}
	if err == secretstore.ErrNotFound {
		return "", nil
	}
	return "", err
}

func runBridgeWithReconnect(ctx context.Context, wsURL string, mcpCommand string, mcpArgs []string, debug bool, reconnect bool) error {
	const initialBackoff = 1 * time.Second
	const maxBackoff = 10 * time.Minute

	backoff := initialBackoff
	attempt := 0

	for {
		err := runBridgeOnce(ctx, wsURL, mcpCommand, mcpArgs, debug)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil && !reconnect {
			return err
		}
		if isAuthError(err) {
			return err
		}

		attempt++
		log.Printf("connection ended: %v", err)
		log.Printf("reconnecting in %s (attempt %d)", backoff, attempt)

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func runBridgeOnce(ctx context.Context, wsURL string, mcpCommand string, mcpArgs []string, debug bool) error {
	cmd := exec.CommandContext(ctx, mcpCommand, mcpArgs...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	go logLines("mcp stderr", stderr)

	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
	}
	conn, resp, err := dialer.DialContext(ctx, wsURL, http.Header{})
	if err != nil {
		if resp != nil {
			return fmt.Errorf("websocket dial failed: %w; status=%s", err, resp.Status)
		}
		return fmt.Errorf("websocket dial failed: %w", err)
	}
	defer conn.Close()

	log.Printf("connected to XiaoZhi MCP endpoint")
	log.Printf("started local MCP server: %s %s", mcpCommand, strings.Join(mcpArgs, " "))

	errCh := make(chan error, 2)
	var writeMu sync.Mutex

	go func() {
		errCh <- forwardWebSocketToMCP(ctx, conn, stdin, debug)
	}()

	go func() {
		errCh <- forwardMCPToWebSocket(ctx, stdout, conn, &writeMu, debug)
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "401 Unauthorized") || strings.Contains(err.Error(), "403 Forbidden")
}

func forwardWebSocketToMCP(ctx context.Context, conn *websocket.Conn, stdin io.WriteCloser, debug bool) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msgType, payload, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read websocket: %w", err)
		}
		if msgType != websocket.TextMessage && msgType != websocket.BinaryMessage {
			if debug {
				log.Printf("<< ignored websocket frame type=%d bytes=%d", msgType, len(payload))
			}
			continue
		}

		if debug {
			log.Printf("<< ws %s", trimForLog(payload))
		}

		if _, err := stdin.Write(append(payload, '\n')); err != nil {
			return fmt.Errorf("write mcp stdin: %w", err)
		}
	}
}

func forwardMCPToWebSocket(ctx context.Context, stdout io.Reader, conn *websocket.Conn, writeMu *sync.Mutex, debug bool) error {
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		if debug {
			log.Printf(">> mcp %s", trimForLog(line))
		}

		writeMu.Lock()
		err := conn.WriteMessage(websocket.TextMessage, line)
		writeMu.Unlock()
		if err != nil {
			return fmt.Errorf("write websocket: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read mcp stdout: %w", err)
	}
	return io.EOF
}

func trimForLog(payload []byte) string {
	const max = 500
	text := string(payload)
	text = strings.ReplaceAll(text, "\n", "\\n")
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}

func logLines(prefix string, r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		log.Printf("%s: %s", prefix, scanner.Text())
	}
}
func xiaoZhiEndpoint(rawURL string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("missing XiaoZhi MCP URL; run stackchan-mcp xiaozhi-store-url or pass --ws")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "wss" && parsed.Scheme != "ws" {
		return "", fmt.Errorf("XiaoZhi MCP URL must start with ws:// or wss://")
	}
	if parsed.Query().Get("token") == "" {
		return "", fmt.Errorf("XiaoZhi MCP URL must be the full Android app URL including token")
	}

	return parsed.String(), nil
}
