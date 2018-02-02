package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Shell struct {
	in *bufio.Reader
}

func NewShell() *Shell {
	return &Shell{in: bufio.NewReader(os.Stdin)}
}

func (s *Shell) scanLine() string {
	line, _ := s.in.ReadString('\n')
	//unsafe but not check error on stdin
	return strings.TrimSpace(line)
}

func (s *Shell) AskCredential() (string, string) {
	username, password := "", ""
	for username == "" {
		fmt.Printf("Enter your username: ")
		username = s.scanLine()
	}

	for password == "" {
		fmt.Printf("Enter your password: ")
		password = s.scanLine()
	}

	return username, password

}

func (s *Shell) Print(msg string) {
	fmt.Printf("%s\n", msg)
}

func (s *Shell) LogAndAuth(uri string) {
	s.Print("Connecting and authenticating to " + uri + "...")
}
