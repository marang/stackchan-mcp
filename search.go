package main

import (
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func searchInternet(args map[string]any) (string, error) {
	rawURL, _ := args["url"].(string)
	query, _ := args["query"].(string)
	maxResults := numberArg(args, "max_results", 5)
	followLinks := boolArg(args, "follow_links", false)
	maxPages := numberArg(args, "max_pages", 5)
	sameHostOnly := boolArg(args, "same_host_only", true)

	rawURL = strings.TrimSpace(rawURL)
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	if maxResults < 1 {
		maxResults = 1
	}
	if maxResults > 10 {
		maxResults = 10
	}
	if maxPages < 1 {
		maxPages = 1
	}
	if maxPages > 12 {
		maxPages = 12
	}
	if rawURL == "" {
		return searchWeb(query, maxResults)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("only http and https URLs are allowed")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("url must include a host")
	}

	if followLinks {
		return searchSiteLinks(parsed, query, maxResults, maxPages, sameHostOnly)
	}

	page, err := fetchPage(parsed.String())
	if err != nil {
		return "", err
	}
	snippets := findSnippets(pageText(page.Body), query, maxResults)
	if len(snippets) == 0 {
		return fmt.Sprintf("No matches for %q found on %s.", query, parsed.String()), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d matches for %q on %s:\n", len(snippets), query, parsed.String())
	for i, snippet := range snippets {
		fmt.Fprintf(&b, "%d. %s\n", i+1, snippet)
	}
	return strings.TrimSpace(b.String()), nil
}

func searchSiteLinks(start *url.URL, query string, maxResults int, maxPages int, sameHostOnly bool) (string, error) {
	queue := []string{start.String()}
	visited := map[string]bool{}
	var matches []pageMatch

	for len(queue) > 0 && len(visited) < maxPages && len(matches) < maxResults {
		current := queue[0]
		queue = queue[1:]
		if visited[current] {
			continue
		}
		visited[current] = true

		page, err := fetchPage(current)
		if err != nil {
			continue
		}

		text := pageText(page.Body)
		snippets := findSnippets(text, query, maxResults-len(matches))
		if len(snippets) > 0 {
			matches = append(matches, pageMatch{
				URL:     current,
				Snippet: snippets[0],
			})
		}

		if len(visited) >= maxPages || len(matches) >= maxResults {
			break
		}

		for _, link := range extractLinks(page.Body, page.URL) {
			if len(queue)+len(visited) >= maxPages*4 {
				break
			}
			if sameHostOnly && !sameHost(page.URL, link) {
				continue
			}
			if !visited[link.String()] {
				queue = append(queue, link.String())
			}
		}
	}

	if len(matches) == 0 {
		return fmt.Sprintf("No matches for %q found in %d visited pages from %s.", query, len(visited), start.String()), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d matches for %q in %d visited pages from %s:\n", len(matches), query, len(visited), start.String())
	for i, match := range matches {
		fmt.Fprintf(&b, "%d. %s\n%s\n", i+1, match.URL, match.Snippet)
	}
	return strings.TrimSpace(b.String()), nil
}

func searchWeb(query string, maxResults int) (string, error) {
	searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	req, err := http.NewRequest(http.MethodGet, searchURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; stackchan-mcp/1.0)")

	body, err := fetchHTTP(req)
	if err != nil {
		return "", err
	}

	results := parseDuckDuckGoResults(body, maxResults)
	if len(results) == 0 {
		return fmt.Sprintf("No web results found for %q.", query), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d web results for %q:\n", len(results), query)
	for i, result := range results {
		fmt.Fprintf(&b, "%d. %s\n%s\n%s\n", i+1, result.Title, result.URL, result.Snippet)
	}
	return strings.TrimSpace(b.String()), nil
}

func fetchHTTP(req *http.Request) (string, error) {
	if err := validatePublicHTTPURL(req.URL); err != nil {
		return "", err
	}
	client := safeHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("website returned %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

type searchResult struct {
	Title   string
	URL     string
	Snippet string
}

type fetchedPage struct {
	URL  *url.URL
	Body string
}

type pageMatch struct {
	URL     string
	Snippet string
}

func parseDuckDuckGoResults(raw string, maxResults int) []searchResult {
	blocks := regexp.MustCompile(`(?is)<div class="result results_links.*?</div>\s*</div>`).FindAllString(raw, -1)
	var results []searchResult

	for _, block := range blocks {
		if len(results) >= maxResults {
			break
		}

		titleMatch := regexp.MustCompile(`(?is)<a[^>]+class="result__a"[^>]+href="([^"]+)"[^>]*>(.*?)</a>`).FindStringSubmatch(block)
		if len(titleMatch) != 3 {
			continue
		}

		snippet := ""
		snippetMatch := regexp.MustCompile(`(?is)<a[^>]+class="result__snippet"[^>]*>(.*?)</a>`).FindStringSubmatch(block)
		if len(snippetMatch) == 2 {
			snippet = cleanupHTML(snippetMatch[1])
		}

		results = append(results, searchResult{
			Title:   cleanupHTML(titleMatch[2]),
			URL:     normalizeDuckDuckGoURL(html.UnescapeString(titleMatch[1])),
			Snippet: snippet,
		})
	}

	return results
}

func cleanupHTML(raw string) string {
	text := regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(raw, " ")
	text = html.UnescapeString(text)
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func normalizeDuckDuckGoURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err == nil {
		if uddg := parsed.Query().Get("uddg"); uddg != "" {
			if decoded, err := url.QueryUnescape(uddg); err == nil {
				return decoded
			}
			return uddg
		}
	}
	return raw
}

func fetchPage(rawURL string) (fetchedPage, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return fetchedPage{}, err
	}
	req.Header.Set("User-Agent", "stackchan-mcp/1.0")

	body, finalURL, err := fetchHTTPWithURL(req)
	if err != nil {
		return fetchedPage{}, err
	}

	return fetchedPage{
		URL:  finalURL,
		Body: body,
	}, nil
}

func fetchHTTPWithURL(req *http.Request) (string, *url.URL, error) {
	if err := validatePublicHTTPURL(req.URL); err != nil {
		return "", nil, err
	}
	client := safeHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("website returned %s", resp.Status)
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "text/plain") {
		return "", nil, fmt.Errorf("unsupported content type %q", resp.Header.Get("Content-Type"))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", nil, err
	}
	return string(body), resp.Request.URL, nil
}

func safeHTTPClient() http.Client {
	return http.Client{
		Timeout: 12 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return validatePublicHTTPURL(req.URL)
		},
	}
}

func validatePublicHTTPURL(u *url.URL) error {
	if u == nil {
		return fmt.Errorf("url is required")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http and https URLs are allowed")
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return fmt.Errorf("url must include a host")
	}
	lowerHost := strings.ToLower(strings.TrimSuffix(host, "."))
	if lowerHost == "localhost" || strings.HasSuffix(lowerHost, ".localhost") {
		return fmt.Errorf("refusing to fetch local host %q", host)
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("resolve %q: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("resolve %q: no addresses", host)
	}
	for _, ip := range ips {
		if blockedHTTPIP(ip) {
			return fmt.Errorf("refusing to fetch private or local address %s for host %q", ip.String(), host)
		}
	}
	return nil
}

func blockedHTTPIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	return ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127
}

func extractLinks(raw string, base *url.URL) []*url.URL {
	linkRe := regexp.MustCompile(`(?is)<a[^>]+href\s*=\s*["']([^"']+)["']`)
	matches := linkRe.FindAllStringSubmatch(raw, -1)
	seen := map[string]bool{}
	var links []*url.URL

	for _, match := range matches {
		if len(match) != 2 {
			continue
		}
		link, err := normalizeLink(match[1], base)
		if err != nil || link == nil {
			continue
		}
		key := link.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		links = append(links, link)
	}

	return links
}

func normalizeLink(raw string, base *url.URL) (*url.URL, error) {
	raw = strings.TrimSpace(html.UnescapeString(raw))
	if raw == "" || strings.HasPrefix(raw, "#") {
		return nil, nil
	}

	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "mailto:") || strings.HasPrefix(lower, "tel:") || strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") {
		return nil, nil
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	link := base.ResolveReference(parsed)
	if link.Scheme != "http" && link.Scheme != "https" {
		return nil, nil
	}
	link.Fragment = ""
	return link, nil
}

func sameHost(a *url.URL, b *url.URL) bool {
	return strings.EqualFold(a.Hostname(), b.Hostname())
}
func pageText(raw string) string {
	text := raw
	text = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func findSnippets(text string, query string, maxResults int) []string {
	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)
	var snippets []string
	offset := 0

	for len(snippets) < maxResults {
		idx := strings.Index(lowerText[offset:], lowerQuery)
		if idx < 0 {
			break
		}
		matchStart := offset + idx
		matchEnd := matchStart + len(lowerQuery)
		snippets = append(snippets, snippet(text, matchStart, matchEnd))
		offset = matchEnd
	}

	return snippets
}

func snippet(text string, matchStart int, matchEnd int) string {
	start := matchStart - 90
	if start < 0 {
		start = 0
	}
	end := matchEnd + 140
	if end > len(text) {
		end = len(text)
	}

	for start > 0 && !isUTF8Boundary(text[start]) {
		start--
	}
	for end < len(text) && !isUTF8Boundary(text[end]) {
		end++
	}

	prefix := ""
	if start > 0 {
		prefix = "..."
	}
	suffix := ""
	if end < len(text) {
		suffix = "..."
	}
	return prefix + strings.TrimSpace(text[start:end]) + suffix
}

func isUTF8Boundary(b byte) bool {
	return b < 0x80 || b >= 0xC0
}
