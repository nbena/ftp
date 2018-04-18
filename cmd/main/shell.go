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

	pb "gopkg.in/cheggaaa/pb.v1"
	// "github.com/vbauerster/mpb"
)

type shell struct {
	in  *bufio.Reader
	out *bufio.Writer
	// progress      *mpb.Progress
	// sreaderChannel chan string
	writerChannel chan *shellOutput
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
		writerChannel: make(chan *shellOutput),
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
		fmt.Fprintf(s.out, "Enter your username: ")
		username = s.scanLine()
	}

	for password == "" {
		fmt.Fprintf(s.out, "Enter your password: ")
		password = s.scanLine()
	}

	return username, password

}

func (s *shell) print(msg string) {
	// fmt.Fprintf(s.out, msg+"\n")
	s.writerChannel <- &shellOutput{
		msg:   msg,
		flush: false,
	}
}

func (s *shell) prompt(location string) {
	// fmt.Printf("ftp:%s>", location)
	s.writerChannel <- &shellOutput{
		msg:   fmt.Sprintf("ftp:%s>", location),
		flush: false,
	}
}

func (s *shell) goodbye() {
	// fmt.Printf("goodbye\n")
	s.writerChannel <- &shellOutput{
		msg:   "goodbye\n",
		flush: false,
	}
}

func (s *shell) printError(msg string, exit bool) {
	fmt.Fprintf(os.Stderr, "%s\n", msg)
	if exit {
		os.Exit(1)
	}
}

func (s *shell) flush() {
	// bufio.NewWriter(os.Stdout).Flush()
	// s.in.ReadLine()
	// s.print("i flushed")
	s.writerChannel <- &shellOutput{
		msg:   "",
		flush: true,
	}
}

func (s *shell) displayProgressBar(max int) *pb.ProgressBar {
	progressBar := pb.New(max)
	return progressBar
	// bar := s.progress.AddBar(int64(max))
	// return bar
}

func (s *shell) writer() chan *shellOutput {
	return s.writerChannel
}

func (s *shell) start() {
	go func() {
		for output := range s.writerChannel {
			// fmt.Printf("i received: %s", output.msg)
			if output.flush {
				// s.out.Flush()
				s.in.ReadLine()
			} else {
				// fmt.Fprint(s.out, output.msg)
				// fmt.Printf("i write it")
				s.out.WriteString(output.msg)
				s.out.Flush()
			}
		}
	}()
}

// func newShell() *shell {
// 	return &shell{
// 		in:            bufio.NewReader(os.Stdin),
// 		readerChannel: make(chan string),
// 		writerChannel: make(chan string),
// 	}
// }
//
// func (s *shell) Channels() (chan string, chan string) {
// 	return s.readerChannel, s.writerChannel
// }
//
// func (s *shell) askForReading() {
// 	line, _ := s.in.ReadString('\n')
// 	//unsafe but not check error on stdin
// 	s.readerChannel <- strings.TrimSpace(line)
// }

// func (s *shell) LogAndAuth(uri string) {
// 	s.Print("Connecting and authenticating to " + uri + "...")
// }
