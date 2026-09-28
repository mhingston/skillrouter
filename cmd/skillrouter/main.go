package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/mhingston/skillrouter/internal/embed"
	"github.com/mhingston/skillrouter/internal/search"
	"github.com/mhingston/skillrouter/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type dirsFlag []string

func (d *dirsFlag) String() string { return strings.Join(*d, string(os.PathListSeparator)) }
func (d *dirsFlag) Set(value string) error {
	*d = append(*d, value)
	return nil
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(0)
	var dirs dirsFlag
	flag.Var(&dirs, "skills-dir", "directory containing Agent Skills; repeat for multiple roots")
	embeddingURL := flag.String("embedding-url", env("SKILLROUTER_EMBEDDING_URL", ""), "OpenAI-compatible embeddings endpoint; unset for local lexical retrieval")
	embeddingModel := flag.String("embedding-model", env("SKILLROUTER_EMBEDDING_MODEL", ""), "embedding model name")
	maxBytes := flag.Int64("max-file-bytes", 1<<20, "maximum SKILL.md/resource file size")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(server.Version)
		return
	}
	if len(dirs) == 0 {
		if raw := os.Getenv("SKILLROUTER_SKILLS_DIRS"); raw != "" {
			for _, item := range filepath.SplitList(raw) {
				if strings.TrimSpace(item) != "" {
					dirs = append(dirs, item)
				}
			}
		}
	}
	if len(dirs) == 0 {
		log.Fatal("no skills roots configured; pass --skills-dir or set SKILLROUTER_SKILLS_DIRS")
	}

	var embedder search.Embedder
	if *embeddingURL != "" || *embeddingModel != "" {
		if *embeddingURL == "" || *embeddingModel == "" {
			log.Fatal("--embedding-url and --embedding-model must be provided together")
		}
		embedder = &embed.OpenAICompatible{URL: *embeddingURL, Model: *embeddingModel, APIKey: os.Getenv("SKILLROUTER_EMBEDDING_API_KEY")}
	}
	engine, err := server.NewEngine(dirs, *maxBytes, embedder)
	if err != nil {
		log.Fatalf("initialize skillrouter: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := engine.MCPServer().Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		log.Fatalf("serve MCP: %v", err)
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
