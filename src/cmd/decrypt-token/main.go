package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/nexus-commerce/user-service/src/nacl"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return errors.New("usage: decrypt-token <encryptedToken>")
	}
	if err := godotenv.Load(".env"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("parse .env: %w", err)
	}

	opener, err := nacl.NewOpener(os.Getenv("NOTIFICATION_PRIVATE_KEY"))
	if err != nil {
		return fmt.Errorf("NOTIFICATION_PRIVATE_KEY: %w", err)
	}
	sealed, err := base64.StdEncoding.DecodeString(os.Args[1])
	if err != nil {
		return fmt.Errorf("decode encryptedToken: %w", err)
	}
	token, err := opener.Open(sealed)
	if err != nil {
		return err
	}

	fmt.Println(string(token))
	return nil
}
