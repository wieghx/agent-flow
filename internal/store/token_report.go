package store

import (
	"context"
	"database/sql"
)

// TokenReportNovel is per-novel token stats with chapter breakdown.
type TokenReportNovel struct {
	Namespace        string         `json:"namespace"`
	Name             string         `json:"name"`
	Title            string         `json:"title"`
	ChapterCount     int            `json:"chapter_count"`
	DoneChapters     int            `json:"chapters_done"`
	PromptTokens     int            `json:"prompt_tokens"`
	CompletionTokens int            `json:"completion_tokens"`
	TotalTokens      int            `json:"total_tokens"`
	AvgChapterTokens int            `json:"avg_chapter_tokens"`
	EstimatedCostUSD float64        `json:"estimated_cost_usd"`
	Chapters         []ChapterEntry `json:"chapters"`
}

// TokenReport is the aggregated token usage summary.
type TokenReport struct {
	NovelCount       int                `json:"novel_count"`
	ChapterCount     int                `json:"chapter_count"`
	DoneChapters     int                `json:"chapters_done"`
	ChaptersWithData int                `json:"chapters_with_tokens"`
	PromptTokens     int                `json:"prompt_tokens"`
	CompletionTokens int                `json:"completion_tokens"`
	TotalTokens      int                `json:"total_tokens"`
	AvgNovelTokens   int                `json:"avg_novel_tokens"`
	AvgChapterTokens int                `json:"avg_chapter_tokens"`
	EstimatedCostUSD float64            `json:"estimated_cost_usd"`
	CostModel        string             `json:"cost_model,omitempty"`
	Novels           []TokenReportNovel `json:"novels"`
}

// BuildTokenReport returns aggregated token usage across all novels.
func (s *SQLStore) BuildTokenReport(ctx context.Context) (*TokenReport, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`
		SELECT n.namespace, n.name, COALESCE(n.title,''), n.chapter_count,
			COALESCE(n.prompt_tokens, 0), COALESCE(n.completion_tokens, 0), COALESCE(n.total_tokens, 0),
			COALESCE(c.num, 0), COALESCE(c.title,''), COALESCE(c.summary,''), COALESCE(c.status,''),
			COALESCE(c.word_count, 0), COALESCE(c.prompt_tokens, 0), COALESCE(c.completion_tokens, 0), COALESCE(c.total_tokens, 0)
		FROM novels n
		LEFT JOIN chapters c ON c.novel_id = n.id
		ORDER BY n.total_tokens DESC, n.updated_at DESC, c.num ASC
	`, `
		SELECT n.namespace, n.name, COALESCE(n.title,''), n.chapter_count,
			COALESCE(n.prompt_tokens, 0), COALESCE(n.completion_tokens, 0), COALESCE(n.total_tokens, 0),
			COALESCE(c.num, 0), COALESCE(c.title,''), COALESCE(c.summary,''), COALESCE(c.status,''),
			COALESCE(c.word_count, 0), COALESCE(c.prompt_tokens, 0), COALESCE(c.completion_tokens, 0), COALESCE(c.total_tokens, 0)
		FROM novels n
		LEFT JOIN chapters c ON c.novel_id = n.id
		ORDER BY n.total_tokens DESC, n.updated_at DESC, c.num ASC
	`))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanTokenReport(rows)
}

func scanTokenReport(rows *sql.Rows) (*TokenReport, error) {
	report := &TokenReport{}
	byKey := map[string]*TokenReportNovel{}
	order := []string{}

	for rows.Next() {
		var ns, name, title string
		var chapterCount, nPrompt, nCompletion, nTotal int
		var chNum, chWord, chPrompt, chCompletion, chTotal int
		var chTitle, chSummary, chStatus string

		if err := rows.Scan(
			&ns, &name, &title, &chapterCount,
			&nPrompt, &nCompletion, &nTotal,
			&chNum, &chTitle, &chSummary, &chStatus,
			&chWord, &chPrompt, &chCompletion, &chTotal,
		); err != nil {
			return nil, err
		}

		key := ns + "/" + name
		novel, ok := byKey[key]
		if !ok {
			novel = &TokenReportNovel{
				Namespace:        ns,
				Name:             name,
				Title:            title,
				ChapterCount:     chapterCount,
				PromptTokens:     nPrompt,
				CompletionTokens: nCompletion,
				TotalTokens:      nTotal,
			}
			byKey[key] = novel
			order = append(order, key)
		}

		if chNum > 0 {
			novel.Chapters = append(novel.Chapters, ChapterEntry{
				Num:              chNum,
				Title:            chTitle,
				Summary:          chSummary,
				Status:           chStatus,
				WordCount:        chWord,
				PromptTokens:     chPrompt,
				CompletionTokens: chCompletion,
				TotalTokens:      chTotal,
			})
			report.ChapterCount++
			if chStatus == ChapterStatusDone {
				novel.DoneChapters++
				report.DoneChapters++
			}
			if chTotal > 0 {
				report.ChaptersWithData++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	report.NovelCount = len(order)
	var novelsWithTokens int
	var globalChapterTokenSum int

	for _, key := range order {
		novel := byKey[key]
		var sum int
		var count int
		for _, ch := range novel.Chapters {
			if ch.TotalTokens > 0 {
				sum += ch.TotalTokens
				count++
				globalChapterTokenSum += ch.TotalTokens
			}
		}
		if count > 0 {
			novel.AvgChapterTokens = sum / count
		}

		report.PromptTokens += novel.PromptTokens
		report.CompletionTokens += novel.CompletionTokens
		report.TotalTokens += novel.TotalTokens
		if novel.TotalTokens > 0 {
			novelsWithTokens++
		}
		report.Novels = append(report.Novels, *novel)
	}

	if novelsWithTokens > 0 {
		report.AvgNovelTokens = report.TotalTokens / novelsWithTokens
	}
	if report.ChaptersWithData > 0 {
		report.AvgChapterTokens = globalChapterTokenSum / report.ChaptersWithData
	}
	return report, nil
}

// BuildTokenReportFromStore returns the report when supported.
func BuildTokenReportFromStore(ctx context.Context, st Store) (*TokenReport, error) {
	if ls, ok := st.(interface {
		BuildTokenReport(context.Context) (*TokenReport, error)
	}); ok && st.Enabled() {
		return ls.BuildTokenReport(ctx)
	}
	return &TokenReport{}, nil
}
