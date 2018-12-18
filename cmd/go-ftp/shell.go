/*
Copyright 2018 Nicola Bena

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	pb "gopkg.in/cheggaaa/pb.v1"
	// "github.com/vbauerster/mpb"
)

type shell struct {
	in  *bufio.Reader
	out *bufio.Writer
	// progress      *mpb.Progress
	// sreaderChannel chan string
	// writerChannel chan *shellOutput
}

type shellOutput struct {
	msg   string
	flush bool
}

func newShell() *shell {
	return &shell{
		in:  bufio.NewReader(os.Stdin),
		out: bufio.NewWriter(os.Stdout),
		// progress: mpb.New(),
		// writerChannel: make(chan *shellOutput),
	}
}

func (s *shell) scanLine() (string, error) {
	// buffered := s.in.Buffered()
	// if reset {
	// 	// s.in.Reset(s.in)
	//
	// 	filee, _ := os.Create("fileee")
	// 	filee.WriteString(fmt.Sprintf("buffered %d\n", buffered))
	// 	if buffered > 0 {
	// 		d, err := s.in.Discard(buffered)
	// 		if err != nil {
	// 			filee.WriteString("Err:" + err.Error() + "\n")
	// 		} else {
	// 			filee.WriteString(fmt.Sprintf("discarded: %d\n", d))
	// 		}
	// 	}
	//
	// }
	// var line string
	// if buffered > 0 {
	line, err := s.in.ReadString('\n')
	// } else {
	// 	line = ""
	// }

	//unsafe but not check error on stdin
	return strings.TrimSpace(line), err
}

// func (s *shell) discard() {
// 	os.Stdin.Seek(0, 2)
// 	os.Stdout.Seek(0, 2)
// }

func (s *shell) askCredential() (string, string, error) {
	username, password := "", ""
	var err error
	loop := true
	for loop == true {
		// fmt.Fprintf(s.out, "Enter your username: ")
		s.print("Enter your username: ")
		username, err = s.scanLine()
		if err != nil || username != "" {
			loop = false
		}
	}

	// necessary to skip the loop if there was
	// an error previously.
	for loop == true && err == nil {
		// fmt.Fprintf(s.out, "Enter your password: ")
		s.print("Enter your password: ")
		password, err = s.scanLine()
		if err != nil || username != "" {
			loop = false
		}
	}

	return username, password, err

}

func (s *shell) print(msg string) {
	//fmt.Fprintf(s.out, msg+"\n")
	s.out.WriteString(msg)
	s.out.Flush()
}

func (s *shell) prompt(location string) {
	s.print(fmt.Sprintf("ftp:%s>", location))
}

func (s *shell) goodbye() {
	s.print("goodbye\n")
}

func (s *shell) printError(msg string, exit bool) {
	fmt.Fprintf(os.Stderr, "%s\n", msg)
	if exit {
		os.Exit(1)
	}
}

func (s *shell) displayProgressBar(max int) *pb.ProgressBar {
	progressBar := pb.New(max)
	return progressBar
	// bar := s.progress.AddBar(int64(max))
	// return bar
}
