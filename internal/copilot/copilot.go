package copilot

import (
	"context"
	"fmt"
	"strings"

	"github.com/Mohamed-Elfardy/pgwatch-copilot/internal/llm"
	"github.com/Mohamed-Elfardy/pgwatch-copilot/internal/metrics"
	"github.com/Mohamed-Elfardy/pgwatch-copilot/internal/profile"
)

type Copilot struct {
	llm     llm.Provider
	querier *metrics.Querier
	profile *profile.Store
	schema  string
}

type Response struct {
	SQL     string
	Columns []string
	Rows    [][]string
	Insight string
}

func New(provider llm.Provider, querier *metrics.Querier, profileStore *profile.Store) (*Copilot, error) {
	schema, err := querier.SchemaContext()
	if err != nil {
		return nil, fmt.Errorf("failed to build schema context: %w", err)
	}
	return &Copilot{
		llm:     provider,
		querier: querier,
		profile: profileStore,
		schema:  schema,
	}, nil
}

func (c *Copilot) Ask(ctx context.Context, question string) (*Response, error) {
	clusterProfile, err := c.profile.Get(c.querier.Dbname())
	if err != nil {
		return nil, fmt.Errorf("failed to load cluster profile: %w", err)
	}

	sql, err := c.generateSQL(ctx, question, clusterProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to generate SQL: %w", err)
	}

	result, err := c.querier.RunQuery(sql)
	if err != nil {
		return &Response{SQL: sql}, fmt.Errorf("query failed: %w", err)
	}

	insight, err := c.generateInsight(ctx, question, sql, result, clusterProfile)
	if err != nil {
		insight = "Could not generate insight."
	}

	return &Response{
		SQL:     sql,
		Columns: result.Columns,
		Rows:    result.Rows,
		Insight: insight,
	}, nil
}

func (c *Copilot) generateSQL(ctx context.Context, question string, p *profile.ClusterProfile) (string, error) {
	system := fmt.Sprintf(`You are an expert PostgreSQL analyst for pgwatch — a PostgreSQL monitoring system.
You help engineers analyze database performance metrics using natural language.

%s

Cluster profile:
- dbname: %s
- Workload type: %s
- Known bottlenecks: %s

Rules:
- Return ONLY the SQL query, no explanation, no markdown, no backticks.
- ALWAYS include WHERE dbname = '%s' in every query.
- Always LIMIT results to 20 rows unless the user asks for more.
- Use proper column aliases for readability.
- For time-based queries, filter recent data using: WHERE time > NOW() - INTERVAL '1 hour'.
- If the question cannot be answered with available tables, return:
  SELECT 'Cannot answer this question with available metrics' AS message;`,
		c.schema,
		p.SysID,
		p.WorkloadType,
		strings.Join(p.HistoricalBottlenecks, ", "),
		p.SysID,
	)

	resp, err := c.llm.Complete(ctx, system, []llm.Message{
		{Role: "user", Content: question},
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Content), nil
}

func (c *Copilot) generateInsight(ctx context.Context, question, sql string, result *metrics.QueryResult, p *profile.ClusterProfile) (string, error) {
	if len(result.Rows) == 0 {
		return "No data found for this query.", nil
	}

	dataPreview := formatPreview(result.Columns, result.Rows)

	prompt := fmt.Sprintf(`The user asked: "%s"
SQL executed: %s

Results:
%s

Cluster workload type: %s

Provide a concise analysis (2-4 sentences) for a database engineer.
Be specific about numbers and highlight anything that looks problematic.`, question, sql, dataPreview, p.WorkloadType)

	resp, err := c.llm.Complete(ctx, "", []llm.Message{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Content), nil
}

func formatPreview(cols []string, rows [][]string) string {
	var sb strings.Builder
	sb.WriteString(strings.Join(cols, " | ") + "\n")
	sb.WriteString(strings.Repeat("-", 60) + "\n")
	limit := len(rows)
	if limit > 10 {
		limit = 10
	}
	for _, row := range rows[:limit] {
		sb.WriteString(strings.Join(row, " | ") + "\n")
	}
	if len(rows) > 10 {
		sb.WriteString(fmt.Sprintf("... and %d more rows\n", len(rows)-10))
	}
	return sb.String()
}
