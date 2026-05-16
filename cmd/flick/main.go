// flick is a terminal-native tool for secure peer-to-peer file transfer and
// chat between machines you control. Connections are encrypted with TLS 1.3
// over QUIC. Identity is managed via Ed25519 keypairs with TOFU verification.
//
// Usage:
//
//	flick init          — generate an identity and start the daemon
//	flick id            — print your fingerprint and randomart
//	flick daemon        — start the background daemon explicitly
//	flick status        — show daemon status and connected peers
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/flick/flick/internal/config"
	"github.com/flick/flick/internal/daemon"
	"github.com/flick/flick/internal/identity"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		// cobra already prints the error; just exit with a non-zero code.
		os.Exit(1)
	}
}

// defaultConfigDir returns ~/.config/flick, creating it if necessary.
func defaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: could not determine home directory:", err)
		os.Exit(1)
	}
	return filepath.Join(home, ".config", "flick")
}

func rootCmd() *cobra.Command {
	var configDir string

	root := &cobra.Command{
		Use:   "flick",
		Short: "Secure terminal file transfer and chat between your machines",
		Long: `flick connects your machines on a local network (or VPN) for fast,
encrypted file transfer and chat — no internet required, no accounts, no servers.

Run 'flick init' to get started.`,
	}

	root.PersistentFlags().StringVar(
		&configDir, "config-dir", defaultConfigDir(),
		"directory for flick config and identity files",
	)

	root.AddCommand(
		initCmd(&configDir),
		idCmd(&configDir),
		daemonCmd(&configDir),
		statusCmd(&configDir),
	)

	return root
}

// initCmd implements 'flick init'.
func initCmd(configDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Generate your identity and start the flick daemon",
		Long: `Creates an Ed25519 keypair for this machine and writes it to the config
directory. The keypair is permanent — your fingerprint is your identity.

If an identity already exists, init does nothing except ensure the daemon is running.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := *configDir
			identityDir := filepath.Join(dir, "identity")

			// Generate a new identity, or load the existing one.
			id, err := loadOrGenerateIdentity(identityDir)
			if err != nil {
				return err
			}

			// Generate config with defaults if it doesn't exist yet.
			if _, err := config.Load(dir); err != nil {
				return fmt.Errorf("initialise config: %w", err)
			}

			fmt.Println("Identity ready.")
			fmt.Println(identity.FormatIdentityBlock(id.PublicKey))
			fmt.Println()

			// Start the daemon if it's not already running.
			fmt.Print("Starting daemon... ")
			if err := daemon.EnsureRunning(dir); err != nil {
				return fmt.Errorf("start daemon: %w", err)
			}
			fmt.Println("done.")
			return nil
		},
	}
}

// idCmd implements 'flick id'.
func idCmd(configDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "id",
		Short: "Print your fingerprint and visual identity",
		Long: `Displays your Ed25519 fingerprint and SSH-style randomart.
Share your fingerprint with peers out-of-band (phone call, Signal, in person)
so they can verify your identity when connecting for the first time.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := identity.Load(filepath.Join(*configDir, "identity"))
			if err != nil {
				return fmt.Errorf("%w\n\nRun 'flick init' to create an identity", err)
			}

			fmt.Println(identity.FormatIdentityBlock(id.PublicKey))
			return nil
		},
	}
}

// daemonCmd implements 'flick daemon'.
func daemonCmd(configDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run the flick daemon in the foreground",
		Long: `Starts the flick daemon, which manages peer connections and handles
incoming file transfers and chat messages.

Normally the daemon is started automatically by other commands. Use this
command to run the daemon explicitly (e.g. in a systemd service or tmux session).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := *configDir
			identityDir := filepath.Join(dir, "identity")

			id, err := identity.Load(identityDir)
			if err != nil {
				return fmt.Errorf("%w\n\nRun 'flick init' first", err)
			}

			cfg, err := config.Load(dir)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			peers, err := config.LoadPeerStore(dir)
			if err != nil {
				return fmt.Errorf("load peer store: %w", err)
			}

			d := daemon.New(id, cfg, peers, dir)
			return d.Run(context.Background())
		},
	}
}

// statusCmd implements 'flick status'.
func statusCmd(configDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status and connected peers",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := *configDir

			if err := daemon.EnsureRunning(dir); err != nil {
				return err
			}

			client, err := daemon.Connect(daemon.SocketPath(dir))
			if err != nil {
				return err
			}
			defer client.Close()

			resp, err := client.Send(daemon.CmdStatus, nil)
			if err != nil {
				return fmt.Errorf("query daemon: %w", err)
			}
			if !resp.OK {
				return fmt.Errorf("daemon error: %s", resp.Error)
			}

			var status daemon.StatusPayload
			if err := json.Unmarshal(resp.Payload, &status); err != nil {
				return fmt.Errorf("parse daemon response: %w", err)
			}

			fmt.Printf("Nickname:    %s\n", status.Nickname)
			fmt.Printf("Fingerprint: %s\n", status.Fingerprint)
			fmt.Printf("Port:        %d\n", status.Port)
			fmt.Printf("Online peers: %d\n", len(status.OnlinePeers))
			for _, p := range status.OnlinePeers {
				fmt.Printf("  • %s  %s\n", p.Nickname, p.Fingerprint[:17]+"...")
			}
			return nil
		},
	}
}

// loadOrGenerateIdentity returns the existing identity if one exists, or
// generates a new one. This lets 'flick init' be safely run multiple times.
func loadOrGenerateIdentity(identityDir string) (*identity.Identity, error) {
	id, err := identity.Load(identityDir)
	if err == nil {
		fmt.Println("Identity already exists — using existing keypair.")
		return id, nil
	}

	fmt.Println("Generating new Ed25519 keypair...")
	id, err = identity.Generate(identityDir)
	if err != nil {
		return nil, fmt.Errorf("generate identity: %w", err)
	}
	fmt.Println("Keypair generated.")
	return id, nil
}
