package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const defaultWebClientTimeout = 45 * time.Second

var htmlTagRE = regexp.MustCompile(`<[^>]+>`)
var spaceRE = regexp.MustCompile(`\s+`)

// WikipediaSearchTool queries Wikipedia (zh/en) for summaries.
type WikipediaSearchTool struct {
	Client *http.Client
}

func (t *WikipediaSearchTool) Name() string { return "wikipedia_search" }
func (t *WikipediaSearchTool) Description() string {
	return `检索维基百科条目摘要（适合历史、人物、朝代、民俗）。
Input: {"query": "唐朝长安", "lang": "zh", "limit": 5}`
}

func (t *WikipediaSearchTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	query := strings.TrimSpace(stringVal(input, "query"))
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	lang := strings.TrimSpace(stringVal(input, "lang"))
	if lang == "" {
		lang = "zh"
	}
	limit := intFromInput(input, "limit", 5)
	if limit <= 0 || limit > 10 {
		limit = 5
	}
	return searchWikipedia(ctx, t.client(), lang, query, limit)
}

// WebSearchTool combines Wikipedia search with optional DuckDuckGo HTML results.
type WebSearchTool struct {
	Client *http.Client
}

func (t *WebSearchTool) Name() string { return "web_search" }
func (t *WebSearchTool) Description() string {
	return `联网检索历史/人文资料（维基百科 + DuckDuckGo 摘要）。
Input: {"query": "北宋汴京 市井风俗", "lang": "zh", "limit": 5}`
}

func (t *WebSearchTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	query := strings.TrimSpace(stringVal(input, "query"))
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	lang := strings.TrimSpace(stringVal(input, "lang"))
	if lang == "" {
		lang = "zh"
	}
	limit := intFromInput(input, "limit", 5)
	if limit <= 0 || limit > 8 {
		limit = 5
	}

	var b strings.Builder
	wiki, err := searchWikipedia(ctx, t.client(), lang, query, limit)
	if err == nil && strings.TrimSpace(wiki) != "" {
		b.WriteString("【维基百科】\n")
		b.WriteString(wiki)
	}
	ddg, err := searchDuckDuckGo(ctx, t.client(), query, limit)
	if err == nil && strings.TrimSpace(ddg) != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("【网页检索】\n")
		b.WriteString(ddg)
	}
	if b.Len() == 0 {
		return "", fmt.Errorf("no results for query: %s", query)
	}
	return b.String(), nil
}

// WebFetchTool downloads a page and extracts readable text.
type WebFetchTool struct {
	Client *http.Client
}

func (t *WebFetchTool) Name() string { return "web_fetch" }
func (t *WebFetchTool) Description() string {
	return `抓取网页正文（去 HTML 标签）。Input: {"url": "https://...", "max_chars": 6000}`
}

func (t *WebFetchTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	rawURL := strings.TrimSpace(stringVal(input, "url"))
	if rawURL == "" {
		return "", fmt.Errorf("url is required")
	}
	maxChars := intFromInput(input, "max_chars", 6000)
	if maxChars <= 0 || maxChars > 20000 {
		maxChars = 6000
	}
	return fetchWebText(ctx, t.client(), rawURL, maxChars)
}

// HistoricalResearchTool runs multi-topic historical background research in one call.
type HistoricalResearchTool struct {
	Client *http.Client
}

func (t *HistoricalResearchTool) Name() string { return "historical_research" }
func (t *HistoricalResearchTool) Description() string {
	return `一键调研历史小说背景（时代/地理/人物/民俗/制度）。
Input: {"era": "唐朝开元年间", "location": "长安", "topics": ["服饰","饮食","官职"], "figures": "李白,杜甫"}`
}

func (t *HistoricalResearchTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	era := strings.TrimSpace(stringVal(input, "era"))
	location := strings.TrimSpace(stringVal(input, "location"))
	figures := strings.TrimSpace(stringVal(input, "figures"))
	if era == "" && location == "" && figures == "" {
		return "", fmt.Errorf("at least one of era, location, figures is required")
	}

	topics := stringSliceInput(input, "topics")
	if len(topics) == 0 {
		topics = []string{"社会制度", "日常生活", "民俗风貌", "历史人物"}
	}

	queries := buildHistoricalQueries(era, location, figures, topics)
	client := t.client()
	lang := "zh"
	if looksLatinHeavy(era + location) {
		lang = "en"
	}

	var sections []string
	sections = append(sections, fmt.Sprintf("# 历史背景调研\n\n- 时代: %s\n- 地点: %s\n- 人物: %s\n", era, location, figures))

	for i, q := range queries {
		if i >= 8 {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		body, err := searchWikipedia(ctx, client, lang, q, 3)
		if err != nil || strings.TrimSpace(body) == "" {
			body, _ = searchDuckDuckGo(ctx, client, q, 3)
		}
		if strings.TrimSpace(body) == "" {
			continue
		}
		sections = append(sections, fmt.Sprintf("## 检索: %s\n\n%s", q, body))
	}
	if len(sections) <= 1 {
		return "", fmt.Errorf("historical research found no results")
	}
	return strings.Join(sections, "\n\n"), nil
}

func (t *WikipediaSearchTool) client() *http.Client {
	if t.Client != nil {
		return t.Client
	}
	return &http.Client{Timeout: defaultWebClientTimeout}
}

func (t *WebSearchTool) client() *http.Client {
	if t.Client != nil {
		return t.Client
	}
	return &http.Client{Timeout: defaultWebClientTimeout}
}

func (t *WebFetchTool) client() *http.Client {
	if t.Client != nil {
		return t.Client
	}
	return &http.Client{Timeout: defaultWebClientTimeout}
}

func (t *HistoricalResearchTool) client() *http.Client {
	if t.Client != nil {
		return t.Client
	}
	return &http.Client{Timeout: defaultWebClientTimeout}
}

func webTools(client *http.Client) []Tool {
	if client == nil {
		client = &http.Client{Timeout: defaultWebClientTimeout}
	}
	return []Tool{
		&WikipediaSearchTool{Client: client},
		&WebSearchTool{Client: client},
		&WebFetchTool{Client: client},
		&HistoricalResearchTool{Client: client},
	}
}

func buildHistoricalQueries(era, location, figures string, topics []string) []string {
	var queries []string
	if era != "" && location != "" {
		queries = append(queries, era+" "+location+" 历史")
	}
	if era != "" {
		queries = append(queries, era+" 社会制度 风俗")
	}
	if location != "" {
		queries = append(queries, location+" 古代 民俗")
	}
	for _, topic := range topics {
		topic = strings.TrimSpace(topic)
		if topic == "" {
			continue
		}
		if era != "" {
			queries = append(queries, era+" "+topic)
		} else if location != "" {
			queries = append(queries, location+" "+topic)
		}
	}
	for _, name := range strings.Split(figures, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			queries = append(queries, name+" 生平 历史")
		}
	}
	return uniqueStrings(queries)
}

func searchWikipedia(ctx context.Context, client *http.Client, lang, query string, limit int) (string, error) {
	if lang == "" {
		lang = "zh"
	}
	api := fmt.Sprintf("https://%s.wikipedia.org/w/api.php", lang)
	searchURL := api + "?action=query&list=search&format=json&utf8=1&srlimit=" + fmt.Sprintf("%d", limit) +
		"&srsearch=" + url.QueryEscape(query)

	body, err := httpGet(ctx, client, searchURL)
	if err != nil {
		return "", err
	}
	var searchResp struct {
		Query struct {
			Search []struct {
				Title   string `json:"title"`
				Snippet string `json:"snippet"`
			} `json:"search"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return "", err
	}
	if len(searchResp.Query.Search) == 0 {
		return "", fmt.Errorf("wikipedia: no hits")
	}

	var lines []string
	for _, hit := range searchResp.Query.Search {
		title := stripHTML(hit.Title)
		snippet := stripHTML(hit.Snippet)
		extract := snippet
		if detail, err := wikipediaExtract(ctx, client, lang, title); err == nil && detail != "" {
			extract = detail
		}
		pageURL := fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", lang, url.PathEscape(strings.ReplaceAll(title, " ", "_")))
		lines = append(lines, fmt.Sprintf("### %s\n来源: %s\n\n%s", title, pageURL, truncateRunes(extract, 1200)))
	}
	return strings.Join(lines, "\n\n"), nil
}

func wikipediaExtract(ctx context.Context, client *http.Client, lang, title string) (string, error) {
	api := fmt.Sprintf("https://%s.wikipedia.org/w/api.php", lang)
	extractURL := api + "?action=query&prop=extracts&explaintext=1&exintro=1&format=json&titles=" + url.QueryEscape(title)
	body, err := httpGet(ctx, client, extractURL)
	if err != nil {
		return "", err
	}
	var resp struct {
		Query struct {
			Pages map[string]struct {
				Extract string `json:"extract"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	for _, page := range resp.Query.Pages {
		return strings.TrimSpace(page.Extract), nil
	}
	return "", fmt.Errorf("no extract")
}

func searchDuckDuckGo(ctx context.Context, client *http.Client, query string, limit int) (string, error) {
	searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	body, err := httpGet(ctx, client, searchURL)
	if err != nil {
		return "", err
	}
	text := string(body)
	var lines []string
	for _, block := range strings.Split(text, `<div class="result`) {
		if len(lines) >= limit {
			break
		}
		title := extractBetween(block, `<a class="result__a"`, "</a>")
		title = stripHTML(title)
		snippet := extractBetween(block, `<a class="result__snippet`, "</a>")
		if snippet == "" {
			snippet = extractBetween(block, `result__snippet">`, "<")
		}
		snippet = stripHTML(snippet)
		href := extractBetween(block, `href="`, `"`)
		if title == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s\n  %s\n  %s", title, snippet, href))
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("duckduckgo: no hits")
	}
	return strings.Join(lines, "\n"), nil
}

func fetchWebText(ctx context.Context, client *http.Client, rawURL string, maxChars int) (string, error) {
	body, err := httpGet(ctx, client, rawURL)
	if err != nil {
		return "", err
	}
	text := stripHTML(string(body))
	text = spaceRE.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("empty page text")
	}
	return truncateRunes(text, maxChars), nil
}

func httpGet(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "agent-flow-research/1.0")
	req.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d for %s", resp.StatusCode, rawURL)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
}

func stripHTML(s string) string {
	s = htmlTagRE.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	return spaceRE.ReplaceAllString(strings.TrimSpace(s), " ")
}

func extractBetween(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	s = s[i+len(start):]
	if idx := strings.Index(s, ">"); idx >= 0 && strings.Contains(start, "<") {
		s = s[idx+1:]
	}
	j := strings.Index(s, end)
	if j < 0 {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(s[:j])
}

func truncateRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "..."
}

func intFromInput(input map[string]interface{}, key string, fallback int) int {
	switch v := input[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case string:
		var n int
		_, _ = fmt.Sscanf(v, "%d", &n)
		if n > 0 {
			return n
		}
	}
	return fallback
}

func stringSliceInput(input map[string]interface{}, key string) []string {
	raw, ok := input[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []interface{}:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case string:
		var parts []string
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				parts = append(parts, p)
			}
		}
		return parts
	default:
		return nil
	}
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func looksLatinHeavy(s string) bool {
	latin, total := 0, 0
	for _, r := range s {
		if r <= 32 {
			continue
		}
		total++
		if r < 128 {
			latin++
		}
	}
	return total > 0 && latin*100/total > 60
}
