package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Mohamed-Elfardy/pgwatch-copilot/internal/copilot"
	"github.com/Mohamed-Elfardy/pgwatch-copilot/internal/llm"
	"github.com/Mohamed-Elfardy/pgwatch-copilot/internal/metrics"
	"github.com/Mohamed-Elfardy/pgwatch-copilot/internal/profile"
	"github.com/spf13/cobra"
)

var (
	connStr   string
	geminiKey string
	groqKey   string
	dbname    string
)

var rootCmd = &cobra.Command{
	Use:   "pgwatch-copilot",
	Short: " AI copilot for pgwatch PostgreSQL monitoring",
	RunE: func(cmd *cobra.Command, args []string) error {
		if connStr == "" {
			connStr = os.Getenv("PGWATCH_CONN")
		}
		if geminiKey == "" {
			geminiKey = os.Getenv("GEMINI_API_KEY")
		}
		if groqKey == "" {
			groqKey = os.Getenv("GROQ_API_KEY")
		}
		if dbname == "" {
			dbname = os.Getenv("PGWATCH_DBNAME")
		}

		if connStr == "" {
			return fmt.Errorf(" connection string required (--conn or PGWATCH_CONN)")
		}
		if dbname == "" {
			return fmt.Errorf(" dbname required (--dbname or PGWATCH_DBNAME)")
		}

		var provider llm.Provider
		if groqKey != "" {
			provider = llm.NewGroqProvider(groqKey)
		} else if geminiKey != "" {
			provider = llm.NewGeminiProvider(geminiKey)
		} else {
			return fmt.Errorf(" API key required (--groq-key or --gemini-key)")
		}

		fmt.Println(" Connecting to pgwatch_metrics database...")
		querier, err := metrics.NewQuerier(connStr, dbname)
		if err != nil {
			return fmt.Errorf(" DB connection failed: %w", err)
		}
		defer querier.Close()
		fmt.Println(" Connected!")

		profileStore, err := profile.NewStore(connStr)
		if err != nil {
			return fmt.Errorf(" Profile store failed: %w", err)
		}
		defer profileStore.Close()

		c, err := copilot.New(provider, querier, profileStore)
		if err != nil {
			return fmt.Errorf(" Failed to init copilot: %w", err)
		}

		fmt.Printf("\n pgwatch Copilot ready! (dbname: %s, LLM: %s)\n", dbname, provider.Name())
		fmt.Println("Type your question or 'exit' to quit.\n")

		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Print("You: ")
			if !scanner.Scan() {
				break
			}
			question := strings.TrimSpace(scanner.Text())
			if question == "" {
				continue
			}
			if question == "exit" || question == "quit" {
				fmt.Println(" Bye!")
				break
			}

			resp, err := c.Ask(context.Background(), question)
			if err != nil {
				fmt.Printf(" Error: %s\n\n", err.Error())
				continue
			}

			fmt.Printf("\n SQL:\n%s\n\n", resp.SQL)

			if len(resp.Rows) > 0 {
				printTable(resp.Columns, resp.Rows)
			} else {
				fmt.Println(" No data returned.")
			}

			if resp.Insight != "" {
				fmt.Printf("\n Insight:\n%s\n\n", resp.Insight)
			}
		}

		return nil
	},
}

func printTable(cols []string, rows [][]string) {
	fmt.Println("Results:")
	fmt.Println(strings.Join(cols, " | "))
	fmt.Println(strings.Repeat("─", 60))
	for _, row := range rows {
		fmt.Println(strings.Join(row, " | "))
	}
	fmt.Printf("(%d rows)\n", len(rows))
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVar(&connStr, "conn", "", "pgwatch_metrics DB connection string")
	rootCmd.Flags().StringVar(&geminiKey, "gemini-key", "", "Google Gemini API key")
	rootCmd.Flags().StringVar(&groqKey, "groq-key", "", "Groq API key")
	rootCmd.Flags().StringVar(&dbname, "dbname", "", "pgwatch monitored database name")
}
