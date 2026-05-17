// bolt — terminal-native P2P chat and file transfer between your machines.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bolt/bolt/internal/config"
	"github.com/bolt/bolt/internal/daemon"
	"github.com/bolt/bolt/internal/identity"
	"github.com/bolt/bolt/internal/transport"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	var configDir string

	root := &cobra.Command{
		Use:   "bolt",
		Short: "Fast terminal chat and file transfer between your machines",
		Long: `bolt connects your machines on a local network for encrypted chat
and file transfer — peer-to-peer, no accounts, no central chat server.

Run 'bolt init' on each machine, then 'bolt connect <ip>' and 'bolt chat <name>'.`,
	}

	root.PersistentFlags().StringVar(
		&configDir, "config-dir", config.DefaultConfigDir(),
		"directory for bolt config and identity files",
	)

	root.AddCommand(
		versionCmd(),
		initCmd(&configDir),
		idCmd(&configDir),
		daemonCmd(&configDir),
		statusCmd(&configDir),
		connectCmd(&configDir),
		chatCmd(&configDir),
	)

	return root
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the bolt version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(transport.BoltVersion)
		},
	}
}

func initCmd(configDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Generate your identity and start the bolt daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := *configDir
			id, err := loadOrGenerateIdentity(filepath.Join(dir, "identity"))
			if err != nil {
				return err
			}
			if _, err := config.Load(dir); err != nil {
				return fmt.Errorf("initialise config: %w", err)
			}
			fmt.Println("Identity ready.")
			fmt.Println(identity.FormatIdentityBlock(id.PublicKey))
			fmt.Println()
			fmt.Print("Starting daemon... ")
			if err := daemon.EnsureRunning(dir); err != nil {
				return fmt.Errorf("start daemon: %w", err)
			}
			fmt.Println("done.")
			return nil
		},
	}
}

func idCmd(configDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "id",
		Short: "Print your fingerprint and visual identity",
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := identity.Load(filepath.Join(*configDir, "identity"))
			if err != nil {
				return fmt.Errorf("%w\n\nRun 'bolt init' to create an identity", err)
			}
			fmt.Println(identity.FormatIdentityBlock(id.PublicKey))
			return nil
		},
	}
}

func daemonCmd(configDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run the bolt daemon in the foreground",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := *configDir
			id, err := identity.Load(filepath.Join(dir, "identity"))
			if err != nil {
				return fmt.Errorf("%w\n\nRun 'bolt init' first", err)
			}
			cfg, err := config.Load(dir)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			peers, err := config.LoadPeerStore(dir)
			if err != nil {
				return fmt.Errorf("load peers: %w", err)
			}
			return daemon.New(id, cfg, peers, dir).Run(context.Background())
		},
	}
}

func statusCmd(configDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status and connected peers",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := *configDir
			if err := daemon.EnsureRunning(dir); err != nil {
				return err
			}
			client, err := daemon.Connect(dir)
			if err != nil {
				return err
			}
			defer client.Close()

			resp, err := client.Send(daemon.CmdStatus, nil)
			if err != nil {
				return err
			}
			if !resp.OK {
				return fmt.Errorf("daemon: %s", resp.Error)
			}

			var status daemon.StatusPayload
			if err := json.Unmarshal(resp.Payload, &status); err != nil {
				return fmt.Errorf("parse status: %w", err)
			}

			fmt.Printf("Nickname:     %s\n", status.Nickname)
			fmt.Printf("Fingerprint:  %s\n", status.Fingerprint)
			fmt.Printf("Port:         %d\n", status.Port)
			fmt.Printf("Online peers: %d\n", len(status.OnlinePeers))
			for _, p := range status.OnlinePeers {
				fmt.Printf("  • %s  %s\n", p.Nickname, p.Fingerprint[:17]+"...")
			}
			return nil
		},
	}
}

func connectCmd(configDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "connect <ip>",
		Short: "Connect to a peer by IP address on the LAN",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := *configDir
			if err := daemon.EnsureRunning(dir); err != nil {
				return err
			}
			client, err := daemon.Connect(dir)
			if err != nil {
				return err
			}
			defer client.Close()

			resp, err := client.Send(daemon.CmdConnect, daemon.ConnectPayload{Address: args[0]})
			if err != nil {
				return err
			}
			if !resp.OK {
				return fmt.Errorf("connect failed: %s", resp.Error)
			}
			fmt.Printf("Connected to %s\n", args[0])
			return nil
		},
	}
}

func chatCmd(configDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "chat <peer>",
		Short: "Open a 1:1 chat session with a connected peer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := *configDir
			peerName := args[0]

			if err := daemon.EnsureRunning(dir); err != nil {
				return err
			}

			subConn, _, err := daemon.Subscribe(dir)
			if err != nil {
				return err
			}
			defer subConn.Close()

			go readChatEvents(subConn)

			fmt.Printf("Chat with %s (Ctrl+C or /quit to exit)\n", peerName)
			fmt.Println(strings.Repeat("─", 40))

			scanner := bufio.NewScanner(os.Stdin)
			for {
				fmt.Print("> ")
				if !scanner.Scan() {
					break
				}
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				if line == "/quit" || line == "/exit" {
					break
				}

				client, err := daemon.Connect(dir)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
					continue
				}
				resp, err := client.Send(daemon.CmdSendChat, daemon.SendChatPayload{
					Peer: peerName,
					Body: line,
				})
				client.Close()
				if err != nil {
					fmt.Fprintf(os.Stderr, "send failed: %v\n", err)
					continue
				}
				if !resp.OK {
					fmt.Fprintf(os.Stderr, "send failed: %s\n", resp.Error)
				}
			}
			return scanner.Err()
		},
	}
}

func readChatEvents(conn net.Conn) {
	for {
		evt, err := daemon.ReadEvent(conn)
		if err != nil {
			return
		}
		if evt.Type != "chat" {
			continue
		}
		var payload daemon.ChatEventPayload
		if err := json.Unmarshal(evt.Payload, &payload); err != nil {
			continue
		}
		ts := payload.SentAt
		if ts == "" {
			ts = time.Now().UTC().Format("15:04:05")
		} else if t, err := time.Parse(time.RFC3339, ts); err == nil {
			ts = t.Local().Format("15:04:05")
		}
		fmt.Printf("\n[%s] %s: %s\n> ", ts, payload.From, payload.Body)
	}
}

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
