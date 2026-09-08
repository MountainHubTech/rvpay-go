package main

// hash-password is an OPERATOR tool for provisioning the initial
// database-managed administrator. It reads a password from stdin (so the
// plaintext never appears in shell history, argv, logs or source control)
// and prints ONLY the Argon2id PHC-format hash for insertion into the users
// table, e.g.:
//
//	go run ./clients/cmd/hash-password < password.txt
//	# then, as the database administrator:
//	INSERT INTO users (name, email, password_hash, user_role)
//	VALUES ('<name>', '<email>', '<printed-hash>', 'USER_ROLE_ADMIN');
//
// The password itself is never printed, stored, or logged.

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/MountainHubTech/rvpay-go/clients/auth"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil && password == "" {
		fmt.Fprintln(os.Stderr, "failed to read password from stdin")
		os.Exit(1)
	}
	password = strings.TrimRight(password, "\r\n")
	if password == "" {
		fmt.Fprintln(os.Stderr, "password must not be empty")
		os.Exit(1)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to hash password")
		os.Exit(1)
	}
	fmt.Println(hash)
}
