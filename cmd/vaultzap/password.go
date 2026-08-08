package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wallacepnts/vaultzap/internal/auth"
	"github.com/wallacepnts/vaultzap/internal/config"
	"github.com/wallacepnts/vaultzap/internal/store"
)

// warnIfUnclaimed says out loud that the setup screen is open. Whoever reaches the port
// first gets to claim the archive, and that window lasts until someone does — worth a line
// in the log, since the port is published on every interface by default.
func warnIfUnclaimed(ctx context.Context, db *store.Store, cfg config.Config) error {
	if cfg.AuthMode() != config.AuthLogin {
		return nil
	}
	credential, err := db.Credential(ctx)
	if err != nil {
		return fmt.Errorf("ler credencial: %w", err)
	}
	if credential.Complete() {
		return nil
	}
	slog.Warn("nenhum acesso cadastrado: abra o endereço abaixo e crie usuário e senha; " +
		"até lá, quem alcançar esta porta primeiro pode cadastrar")
	return nil
}

// Prints a new password on stdout instead of writing it anywhere: whoever runs this is at
// a terminal, and it is the way back in when the password was lost. The username is kept.
func runResetPassword() error {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	ctx := context.Background()
	db, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	credential, err := db.Credential(ctx)
	if err != nil {
		return fmt.Errorf("ler credencial: %w", err)
	}
	if !credential.Complete() {
		return fmt.Errorf("nenhum acesso cadastrado ainda: abra o VaultZap no navegador e crie usuário e senha")
	}

	password := auth.GeneratePassword()
	hashed, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	hashed.Username = credential.Username
	if err := db.SetCredential(ctx, store.Credential(hashed)); err != nil {
		return fmt.Errorf("gravar credencial: %w", err)
	}
	fmt.Printf("usuário: %s\nnova senha: %s\n", credential.Username, password)
	return nil
}
