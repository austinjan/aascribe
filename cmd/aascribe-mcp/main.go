// aascribe-mcp publishes one explicit project index over authenticated MCP HTTP.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/austinjan/aascribe/internal/knowledge"
	"github.com/austinjan/aascribe/internal/mcpserver"
	"github.com/austinjan/aascribe/internal/projectask"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stderr); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("aascribe-mcp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "Required: dedicated directory of indexed files to publish")
	listen := flags.String("listen", "127.0.0.1:8080", "Listen address; non-loopback requires TLS")
	origins := flags.String("origins", "", "Comma-separated exact allowed Origin values; absent Origin is accepted")
	cert := flags.String("tls-cert", "", "TLS certificate PEM file")
	key := flags.String("tls-key", "", "TLS private key PEM file")
	node := flags.String("node", "node", "Node.js executable for the isolated pi worker")
	worker := flags.String("pi-worker", "runtime/pi/worker.mjs", "Path to the pi SDK worker")
	provider := flags.String("pi-provider", "google", "Server-side pi model provider")
	model := flags.String("pi-model", "gemini-2.5-flash", "Server-side pi model ID")
	modelsFile := flags.String("pi-models-file", "", "Optional explicit pi models.json; no automatic model/config discovery")
	authFile := flags.String("pi-auth-file", "", "Optional dedicated pi OAuth auth.json for openai-codex subscription access")
	codexAuthFile := flags.String("pi-codex-auth-file", "", "Optional existing Codex-managed auth.json; refresh remains owned by Codex CLI")
	codexCommand := flags.String("codex-command", "codex", "Codex CLI executable for managed auth refresh")
	timeout := flags.Duration("query-timeout", 120*time.Second, "Maximum time per question (1s to 5m)")
	concurrency := flags.Int("query-concurrency", 1, "Maximum concurrent pi workers (1 to 8)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *root == "" || flags.NArg() != 0 {
		return fmt.Errorf("specify --root <published-project-directory>; token comes from AASCRIBE_MCP_TOKEN")
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	ip := net.ParseIP(host)
	if (*cert == "") != (*key == "") {
		return fmt.Errorf("--tls-cert and --tls-key must be supplied together")
	}
	if (ip == nil || !ip.IsLoopback()) && *cert == "" {
		return fmt.Errorf("non-loopback listeners require TLS; use 127.0.0.1 behind an HTTPS tunnel")
	}
	catalog, err := knowledge.OpenForIndexing(*root)
	if err != nil {
		return err
	}
	managedCommand := ""
	if *codexAuthFile != "" {
		if *authFile != "" {
			catalog.Close()
			return fmt.Errorf("choose pi-auth-file or pi-codex-auth-file")
		}
		*authFile = *codexAuthFile
		managedCommand = *codexCommand
	}
	runner, err := projectask.New(catalog, projectask.Config{Node: *node, Worker: *worker, Provider: *provider, Model: *model, ModelsFile: *modelsFile, AuthFile: *authFile, CodexCommand: managedCommand, Timeout: *timeout, Concurrency: *concurrency})
	if err != nil {
		catalog.Close()
		return err
	}
	defer runner.Close()
	handler, err := mcpserver.Handler(runner, os.Getenv("AASCRIBE_MCP_TOKEN"), strings.Split(*origins, ","))
	if err != nil {
		return err
	}
	server := &http.Server{Addr: *listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: *timeout + 10*time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024, BaseContext: func(net.Listener) context.Context { return ctx }}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdownCtx)
		case <-done:
		}
	}()
	fmt.Fprintf(stderr, "aascribe MCP listening on %s/mcp (ask_project and index_project)\n", *listen)
	if *cert != "" {
		err = server.ListenAndServeTLS(*cert, *key)
	} else {
		err = server.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
