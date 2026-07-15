// Command dewindex runs the Dew history indexer sidecar (product P1e).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dewnetwork/dew/indexer"
	"github.com/dewnetwork/dew/version"
)

func main() {
	// version before flag.Parse so `dewindex version` works without other flags.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "-version", "--version", "-v":
			fmt.Println(version.Line("dewindex"))
			return
		}
	}

	rpcURL := flag.String("rpc", envOr("INDEXER_RPC_URL", "http://127.0.0.1:8545"), "Dew JSON-RPC HTTP URL")
	dbPath := flag.String("db", envOr("INDEXER_DB", "indexer.db"), "SQLite database path")
	listen := flag.String("http.addr", envOr("INDEXER_LISTEN", "127.0.0.1:8550"), "HTTP listen address")
	start := flag.Uint64("start-block", 0, "first block when DB is empty")
	poll := flag.Duration("poll", time.Second, "tip poll interval after catch-up")
	batch := flag.Int("batch", 64, "max blocks per poll tick")
	verifyToken := flag.String("verify-token", envOr("INDEXER_VERIFY_TOKEN", ""), "optional token for POST /v1/contract (empty = open)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Line("dewindex"))
		return
	}

	cfg := indexer.Config{
		RPCURL:           *rpcURL,
		SQLitePath:       *dbPath,
		ListenAddr:       *listen,
		StartBlock:       *start,
		PollInterval:     *poll,
		MaxBlocksPerTick: *batch,
		VerifyToken:      *verifyToken,
	}

	ix, err := indexer.New(cfg)
	if err != nil {
		log.Fatalf("indexer open: %v", err)
	}
	defer ix.Close()

	srv, err := indexer.NewServer(ix, cfg.ListenAddr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		log.Printf("dewindex %s ingest rpc=%s db=%s", version.Version, cfg.RPCURL, cfg.SQLitePath)
		if err := ix.RunIngest(ctx); err != nil && ctx.Err() == nil {
			log.Printf("ingest stopped: %v", err)
		}
	}()

	go func() {
		log.Printf("dewindex HTTP %s", srv.ListenURL())
		if err := srv.Start(); err != nil {
			log.Printf("http: %v", err)
			cancel()
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println("shutting down…")
	cancel()
	_ = srv.Close()
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
