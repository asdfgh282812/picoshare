package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	gorilla "github.com/mtlynch/gorilla-handlers"

	"github.com/mtlynch/picoshare/garbagecollect"
	"github.com/mtlynch/picoshare/handlers"
	"github.com/mtlynch/picoshare/handlers/auth/oidcauth"
	"github.com/mtlynch/picoshare/space"
	"github.com/mtlynch/picoshare/store/sqlite"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Llongfile)
	log.Print("starting picoshare server")

	dbPath := flag.String("db", "data/store.db", "path to database")
	resetOIDC := flag.Bool("reset-oidc", false, "clear the configured identity provider and print a new setup token, then exit")
	flag.Parse()

	dbDir := filepath.Dir(*dbPath)

	ensureDirExists(dbDir)

	store := sqlite.New(sqlite.Params{
		Path:                  *dbPath,
		OptimizeForLitestream: isLitestreamEnabled(),
		Now:                   time.Now,
	})

	if *resetOIDC {
		runResetOIDC(&store)
		return
	}

	if err := ensureSetupToken(&store); err != nil {
		log.Fatalf("failed to prepare setup token: %v", err)
	}

	identityProvider := oidcauth.New(&store, time.Now)

	spaceChecker := space.NewChecker(*dbPath, &store)

	collector := garbagecollect.NewCollector(store, time.Now)
	gc := garbagecollect.NewScheduler(&collector, 7*time.Hour)
	gc.StartAsync()

	server := handlers.New(handlers.Params{
		IdentityProvider: identityProvider,
		Store:            &store,
		CheckSpace:       spaceChecker.Check,
		Collector:        &collector,
		Now:              time.Now,
	})

	// CrossOriginProtection rejects non-safe cross-origin requests to prevent CSRF.
	protectedRouter := http.NewCrossOriginProtection().Handler(server.Router())
	h := gorilla.LoggingHandler(os.Stdout, protectedRouter)
	if os.Getenv("PS_BEHIND_PROXY") != "" {
		h = gorilla.ProxyIPHeadersHandler(h)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "4001"
	}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}

	stop := setupSignalHandler()
	httpSrv := http.Server{Addr: fmt.Sprintf(":%s", port), Handler: h}
	go func() {
		log.Printf("listening on http://%s:%s", hostname, port)
		log.Printf("http server exit: %s", httpSrv.ListenAndServe())
	}()
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = httpSrv.Shutdown(ctx)
	cancel()
	if err != nil {
		log.Fatal(err)
	}
}

// ensureSetupToken prints a fresh setup token to the log whenever PicoShare
// starts without a configured identity provider, so an administrator can
// complete setup at /setup. It's a no-op once setup is complete.
func ensureSetupToken(store *sqlite.Store) error {
	needsSetup, err := store.NeedsSetup()
	if err != nil {
		return err
	}
	if !needsSetup {
		return nil
	}

	token, err := store.GenerateSetupToken()
	if err != nil {
		return err
	}
	log.Printf("=================================================================")
	log.Printf("PicoShare has no identity provider configured yet.")
	log.Printf("Visit /setup and enter this one-time setup token: %s", token.String())
	log.Printf("=================================================================")
	return nil
}

// runResetOIDC clears PicoShare's identity provider configuration and prints
// a new setup token, for administrators locked out by a misconfiguration.
func runResetOIDC(store *sqlite.Store) {
	if err := store.ClearOIDCSettings(); err != nil {
		log.Fatalf("failed to clear OIDC settings: %v", err)
	}
	token, err := store.GenerateSetupToken()
	if err != nil {
		log.Fatalf("failed to generate a new setup token: %v", err)
	}
	log.Printf("Cleared identity provider configuration.")
	log.Printf("Setup token: %s", token.String())
	log.Printf("Start PicoShare normally and visit /setup to reconfigure single sign-on.")
}

func ensureDirExists(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.Mkdir(dir, os.ModePerm); err != nil {
			panic(err)
		}
	}
}

func isLitestreamEnabled() bool {
	return os.Getenv("LITESTREAM_BUCKET") != ""
}

func setupSignalHandler() <-chan struct{} {
	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()
	return stop
}
