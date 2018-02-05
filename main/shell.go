// Copyright 2018 nbena
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type shell struct {
	in *bufio.Reader
}

func newshell() *shell {
	return &shell{in: bufio.NewReader(os.Stdin)}
}

func (s *shell) scanLine() string {
	line, _ := s.in.ReadString('\n')
	//unsafe but not check error on stdin
	return strings.TrimSpace(line)
}

func (s *shell) askCredential() (string, string) {
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

func (s *shell) print(msg string) {
	fmt.Printf("%s\n", msg)
}

// func (s *shell) LogAndAuth(uri string) {
// 	s.Print("Connecting and authenticating to " + uri + "...")
// }
