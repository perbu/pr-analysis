package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/perbu/pr-analyzer/downloader"
	"github.com/perbu/pr-analyzer/processor"
	"github.com/perbu/pr-analyzer/query"
)

func main() {
	var (
		downloadCmd   = flag.NewFlagSet("download", flag.ExitOnError)
		queryCmd      = flag.NewFlagSet("query", flag.ExitOnError)
		processCmd    = flag.NewFlagSet("process-prs", flag.ExitOnError)
		synthesizeCmd = flag.NewFlagSet("synthesize", flag.ExitOnError)

		// Download flags
		token = downloadCmd.String("token", "", "GitHub personal access token")
		owner = downloadCmd.String("owner", "", "Repository owner")
		repo  = downloadCmd.String("repo", "", "Repository name")

		// Query flags
		authors = queryCmd.String("authors", "", "Comma-separated list of authors to filter")
		output  = queryCmd.String("output", "stdout", "Output format: stdout, json, csv")

		// Process flags
		geminiKey   = processCmd.String("key", "", "Gemini API key")
		geminiModel = processCmd.String("model", "gemini-2.5-flash", "Gemini model to use")

		// Synthesize flags
		synthKey   = synthesizeCmd.String("key", "", "Gemini API key")
		synthModel = synthesizeCmd.String("model", "gemini-2.5-flash", "Gemini model to use")
	)

	if len(os.Args) < 2 {
		fmt.Println("Usage: pr-analyzer <command> [options]")
		fmt.Println("Commands:")
		fmt.Println("  download     - Download all PRs from repository")
		fmt.Println("  query        - Query downloaded PRs for author comments")
		fmt.Println("  process-prs  - Process PRs with Gemini to extract learnings")
		fmt.Println("  synthesize   - Synthesize all learnings into a style guide")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "download":
		downloadCmd.Parse(os.Args[2:])
		if *token == "" {
			*token = os.Getenv("GITHUB_TOKEN")
			if *token == "" {
				log.Fatal("GitHub token required: use -token flag or GITHUB_TOKEN env var")
			}
		}
		if *owner == "" {
			log.Fatal("Repository owner required: use -owner flag")
		}
		if *repo == "" {
			log.Fatal("Repository name required: use -repo flag")
		}

		ctx := context.Background()
		d := downloader.New(*token, *owner, *repo)
		if err := d.DownloadAll(ctx); err != nil {
			log.Fatalf("Download failed: %v", err)
		}

	case "query":
		queryCmd.Parse(os.Args[2:])
		if *authors == "" {
			log.Fatal("Authors required: use -authors flag")
		}

		q := query.New()
		results, err := q.FilterByAuthors(*authors, *output)
		if err != nil {
			log.Fatalf("Query failed: %v", err)
		}
		fmt.Println(results)

	case "process-prs":
		processCmd.Parse(os.Args[2:])
		if *geminiKey == "" {
			*geminiKey = os.Getenv("GEMINI_API_KEY")
			if *geminiKey == "" {
				log.Fatal("Gemini API key required: use -key flag or GEMINI_API_KEY env var")
			}
		}

		// Check for model from environment if not provided via flag
		if *geminiModel == "gemini-2.5-flash" {
			if envModel := os.Getenv("GEMINI_MODEL"); envModel != "" {
				*geminiModel = envModel
			}
		}

		ctx := context.Background()
		proc, err := processor.New(*geminiKey, *geminiModel)
		if err != nil {
			log.Fatalf("Failed to create processor: %v", err)
		}
		defer proc.Close()

		if err := proc.ProcessAllPRs(ctx); err != nil {
			log.Fatalf("Processing failed: %v", err)
		}

	case "synthesize":
		synthesizeCmd.Parse(os.Args[2:])
		if *synthKey == "" {
			*synthKey = os.Getenv("GEMINI_API_KEY")
			if *synthKey == "" {
				log.Fatal("Gemini API key required: use -key flag or GEMINI_API_KEY env var")
			}
		}

		// Check for model from environment if not provided via flag
		if *synthModel == "gemini-2.5-flash" {
			if envModel := os.Getenv("GEMINI_MODEL"); envModel != "" {
				*synthModel = envModel
			}
		}

		ctx := context.Background()
		proc, err := processor.New(*synthKey, *synthModel)
		if err != nil {
			log.Fatalf("Failed to create processor: %v", err)
		}
		defer proc.Close()

		if err := proc.SynthesizeStyleGuide(ctx); err != nil {
			log.Fatalf("Synthesis failed: %v", err)
		}

	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
