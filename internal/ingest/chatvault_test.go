package ingest

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

const chatVaultTxt = `17/11/2022 23:52 - Dani Arcanjo: Oi
17/11/2022 23:53 - Wallace Pontes: Oi, tudo bem?
17/11/2022 23:54 - Dani Arcanjo: Tudo!
`

func buildChatVaultFolder(t *testing.T, uuid string, comFoto bool) (raiz, folder string) {
	t.Helper()
	raiz = t.TempDir()
	folder = filepath.Join(raiz, uuid)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatalf("criar pasta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(folder, "_chat.txt"), []byte(chatVaultTxt), 0o644); err != nil {
		t.Fatalf("escrever _chat.txt: %v", err)
	}
	if comFoto {
		if err := os.WriteFile(filepath.Join(folder, "profile-image.jpg"), []byte("fake-jpeg"), 0o644); err != nil {
			t.Fatalf("escrever foto: %v", err)
		}
	}
	return raiz, folder
}

const uuidChatVault = "8c592fcc-5b8e-44f7-89c2-968796bb97fa"

func TestChatVault_nameInferredFromSenders(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	importText(t, s, "Conversa do WhatsApp com Ana.txt",
		"26/07/2026 09:00 - Ana: oi\n26/07/2026 09:01 - Wallace Pontes: oi\n")

	_, folder := buildChatVaultFolder(t, uuidChatVault, false)
	if _, err := ImportFile(ctx, s, folder, uuidChatVault, t.TempDir(), ""); err != nil {
		t.Fatalf("ImportFile: %v", err)
	}

	var n int
	s.DB().QueryRow(`SELECT COUNT(*) FROM chats WHERE name = 'Dani Arcanjo'`).Scan(&n)
	if n != 1 {
		names, _ := s.DB().Query(`SELECT name FROM chats`)
		defer names.Close()
		var list []string
		for names.Next() {
			var name string
			names.Scan(&name)
			list = append(list, name)
		}
		t.Errorf("esperava um chat 'Dani Arcanjo', chats no banco: %v", list)
	}
	if _, err := s.DB().Exec(`SELECT 1`); err != nil {
		t.Fatal(err)
	}
}

func TestChatVault_keepsUUIDWithoutReference(t *testing.T) {
	s := openTestStore(t)
	_, folder := buildChatVaultFolder(t, uuidChatVault, false)

	if _, err := ImportFile(context.Background(), s, folder, uuidChatVault, t.TempDir(), ""); err != nil {
		t.Fatalf("ImportFile: %v", err)
	}
	var name string
	if err := s.DB().QueryRow(`SELECT name FROM chats`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != uuidChatVault {
		t.Errorf("nome = %q, esperado o UUID (sem referência de quem sou eu)", name)
	}
}

func TestChatVault_profilePhotoBecomesAvatar(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	mediaDir := t.TempDir()
	_, folder := buildChatVaultFolder(t, uuidChatVault, true)

	if _, err := ImportFile(ctx, s, folder, uuidChatVault, mediaDir, ""); err != nil {
		t.Fatalf("ImportFile: %v", err)
	}

	var id int64
	var avatar sql.NullString
	if err := s.DB().QueryRow(`SELECT id, avatar_path FROM chats`).Scan(&id, &avatar); err != nil {
		t.Fatal(err)
	}
	if !avatar.Valid || avatar.String == "" {
		t.Fatal("avatar_path vazio: a foto de perfil não foi importada")
	}
	if _, err := os.Stat(filepath.Join(mediaDir, avatar.String)); err != nil {
		t.Errorf("arquivo do avatar não existe em MEDIA_DIR: %v", err)
	}
}

func TestChatVault_doesNotOverwriteUserAvatar(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	mediaDir := t.TempDir()
	_, folder := buildChatVaultFolder(t, uuidChatVault, true)

	if _, err := ImportFile(ctx, s, folder, uuidChatVault, mediaDir, ""); err != nil {
		t.Fatalf("primeiro import: %v", err)
	}
	if _, err := s.DB().Exec(`UPDATE chats SET avatar_path = 'avatars/escolhido.png'`); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportFile(ctx, s, folder, uuidChatVault, mediaDir, ""); err != nil {
		t.Fatalf("reimport: %v", err)
	}

	var avatar string
	if err := s.DB().QueryRow(`SELECT avatar_path FROM chats`).Scan(&avatar); err != nil {
		t.Fatal(err)
	}
	if avatar != "avatars/escolhido.png" {
		t.Errorf("avatar_path = %q, o reimport sobrescreveu a escolha do usuário", avatar)
	}
}
