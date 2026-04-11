package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/synca/daemon/internal/auth"
	"github.com/synca/daemon/internal/config"
	"github.com/synca/daemon/internal/server"
	"github.com/synca/daemon/internal/sync"
)

var rootCmd = &cobra.Command{
	Use:     "synca",
	Short:   "Synca — cloud sync daemon for Linux",
	Version: "0.2.0",
}

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the sync daemon",
	RunE:  runDaemon,
}

var connectCmd = &cobra.Command{
	Use:   "connect google-drive",
	Short: "Authenticate with Google Drive via OAuth2",
	Args:  cobra.ExactArgs(1),
	RunE:  runConnect,
}

var watchCmd = &cobra.Command{
	Use:   "watch [path]",
	Short: "Add a folder to sync",
	Args:  cobra.ExactArgs(1),
	RunE:  runWatch,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status",
	RunE:  runStatus,
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	rootCmd.AddCommand(daemonCmd, connectCmd, watchCmd, statusCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runDaemon(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown on SIGINT/SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Info().Msg("Shutting down Synca daemon...")
		cancel()
	}()

	engine, err := sync.NewEngine(cfg)
	if err != nil {
		return err
	}

	ws := server.NewWebSocketServer(engine)

	log.Info().Str("addr", cfg.WSAddr).Msg("Synca daemon started")

	go func() {
		if err := ws.Start(cfg.WSAddr); err != nil {
			log.Error().Err(err).Msg("WebSocket server error")
		}
	}()

	return engine.Run(ctx)
}

func runConnect(cmd *cobra.Command, args []string) error {
	provider := args[0]
	if provider != "google-drive" {
		log.Error().Msg("Only 'google-drive' is supported in this version")
		return nil
	}
	return auth.RunOAuthFlow()
}

func runWatch(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	path := args[0]
	cfg.AddWatchPath(path)
	if err := cfg.Save(); err != nil {
		return err
	}
	log.Info().Str("path", path).Msg("Watch path added. Restart daemon to apply.")
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Connects to running daemon via WebSocket and prints status
	return server.PrintStatus()
}
