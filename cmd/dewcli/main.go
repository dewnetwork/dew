// Command dewcli is the Dew wallet and utility CLI.
//
// Phase A1: create encrypted wallets and list addresses.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/dewnetwork/dew/crypto/wallet"
	"golang.org/x/term"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "dewcli: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return fmt.Errorf("missing command")
	}

	// Global flags before subcommand.
	var keystoreDir string
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--keystore" && i+1 < len(args):
			keystoreDir = args[i+1]
			i++
		case strings.HasPrefix(a, "--keystore="):
			keystoreDir = strings.TrimPrefix(a, "--keystore=")
		case a == "-h" || a == "--help" || a == "help":
			printUsage()
			return nil
		default:
			rest = append(rest, args[i:]...)
			i = len(args)
		}
	}
	if len(rest) == 0 {
		printUsage()
		return fmt.Errorf("missing command")
	}

	store, err := wallet.Open(keystoreDir)
	if err != nil {
		return err
	}

	switch rest[0] {
	case "wallet":
		return cmdWallet(store, rest[1:])
	case "version":
		fmt.Println("dewcli 0.1.0 (phase A1)")
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", rest[0])
	}
}

func cmdWallet(store *wallet.Store, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: dewcli wallet <create|list>")
	}
	switch args[0] {
	case "create":
		return walletCreate(store, args[1:])
	case "list":
		return walletList(store)
	default:
		return fmt.Errorf("unknown wallet subcommand %q (want create|list)", args[0])
	}
}

func walletCreate(store *wallet.Store, args []string) error {
	pass, err := passphraseFromArgsOrPrompt(args, true)
	if err != nil {
		return err
	}
	addr, err := store.Create(pass)
	if err != nil {
		return err
	}
	fmt.Printf("Created wallet\n")
	fmt.Printf("  Address:  %s\n", addr.Hex())
	fmt.Printf("  Keystore: %s\n", store.Dir())
	return nil
}

func walletList(store *wallet.Store) error {
	list := store.List()
	if len(list) == 0 {
		fmt.Printf("No wallets in %s\n", store.Dir())
		return nil
	}
	fmt.Printf("Wallets in %s:\n", store.Dir())
	for i, a := range list {
		fmt.Printf("  %d. %s\n", i+1, a.Hex())
	}
	return nil
}

// passphraseFromArgsOrPrompt accepts --password=... (tests/CI) or prompts twice.
func passphraseFromArgsOrPrompt(args []string, confirm bool) (string, error) {
	for _, a := range args {
		if strings.HasPrefix(a, "--password=") {
			return strings.TrimPrefix(a, "--password="), nil
		}
		if a == "--password" {
			return "", fmt.Errorf("--password requires a value (use --password=...)")
		}
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("no TTY for password prompt; pass --password=...")
	}

	fmt.Fprint(os.Stderr, "Passphrase: ")
	b1, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if confirm {
		fmt.Fprint(os.Stderr, "Confirm passphrase: ")
		b2, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		if string(b1) != string(b2) {
			return "", fmt.Errorf("passphrases do not match")
		}
	}
	if len(b1) == 0 {
		return "", fmt.Errorf("passphrase must not be empty")
	}
	return string(b1), nil
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `dewcli — Dew wallet CLI

Usage:
  dewcli [--keystore <dir>] wallet create [--password=<pass>]
  dewcli [--keystore <dir>] wallet list
  dewcli version

Environment:
  DEW_KEYSTORE   Default keystore directory (default: ~/.dew/keystore)

`)
}
