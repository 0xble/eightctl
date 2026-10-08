package client

import (
	"os"
	"testing"

	"github.com/0xble/eightsleep/internal/tokencache"
	"github.com/99designs/keyring"
)

// Authentication writes tokens, so isolate both backends from persistent stores.
func TestMain(m *testing.M) {
	primary := keyring.NewArrayKeyring(nil)
	file := keyring.NewArrayKeyring(nil)

	restorePrimary := tokencache.SetOpenKeyringForTest(func() (keyring.Keyring, error) {
		return primary, nil
	})
	restoreFile := tokencache.SetOpenFileKeyringForTest(func() (keyring.Keyring, error) {
		return file, nil
	})

	code := m.Run()

	restoreFile()
	restorePrimary()
	os.Exit(code)
}
