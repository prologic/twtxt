package main

import (
	"encoding/base32"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/blake2b"
)

func fastHash(s string) string {
	sum := blake2b.Sum256([]byte(s))

	// Base32 is URL-safe, unlike Base64, and shorter than hex.
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	hash := strings.ToLower(encoding.EncodeToString(sum[:]))

	return hash
}

func main() {
	data, _ := io.ReadAll(os.Stdin)
	fmt.Print(fastHash(string(data)))
}
