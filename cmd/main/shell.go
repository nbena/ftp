/* ftp
   Copyright (C) 2018 Nicola Bena

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/vbauerster/mpb"
)

type shell struct {
	in       *bufio.Reader
	progress *mpb.Progress
}

func newshell() *shell {
	return &shell{
		in:       bufio.NewReader(os.Stdin),
		progress: mpb.New(),
	}
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

func (s *shell) prompt(location string) {
	fmt.Printf("ftp:%s>", location)
}

func (s *shell) goodbye() {
	fmt.Printf("goodbye\n")
}

func (s *shell) printError(msg string, exit bool) {
	fmt.Fprintf(os.Stderr, "%s\n", msg)
	if exit {
		os.Exit(1)
	}
}

func (s *shell) displayProgressBar(max int) *mpb.Bar {
	// progressBar := pb.New(max)
	// return progressBar
	bar := s.progress.AddBar(int64(max))
	return bar
}

// func (s *shell) LogAndAuth(uri string) {
// 	s.Print("Connecting and authenticating to " + uri + "...")
// }
