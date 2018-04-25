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
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/nbena/ftp"
	pb "gopkg.in/cheggaaa/pb.v1"
)

func getConn() (*ftp.Conn, *ftp.Response, error) {
	// USER MUST HAVE TO SPECIFY SKIP VERIFY EVEN
	// IF HE DOESN'T WANT TO CONNECT USING TLS,
	// BECAUSE IT MAY WANT TO USE IT LATER.
	return ftp.DialAndAuthenticate(remote,
		&ftp.Config{
			TLSOption: &ftp.TLSOption{
				AllowSSL:        allowSSL3,
				SkipVerify:      skipVerify,
				AuthTLSOnFirst:  authTLSOnFirst,
				ContinueIfNoSSL: continueIfNoTLS,
				ImplicitTLS:     implicitTLS,
			},
			DefaultMode: ftpDefaultMode,
			LocalIP:     localIPParsed,
			LocalPort:   localPortParsed,
			Username:    username,
			Password:    password,
		})
}

func main() {

	parseFlags()

	quitChan := make(chan os.Signal)
	signal.Notify(quitChan, syscall.SIGINT, syscall.SIGSTOP)

	if showCiphers {
		ciphers := ftp.CipherSuitesString(allowWeakHash)
		for _, cipher := range ciphers {
			fmt.Printf("%s\n", cipher)
		}
		os.Exit(0)
	}

	if len(parsedCommands) > 0 {
		for _, v := range parsedCommands {
			if v.cmd == commandQuit.cmd {
				// execute and exit.

			}
		}
	}

	shell := newShell()
	if username == "" || password == "" {
		username, password = shell.askCredential()
	}
	conn, response, err := getConn()
	if err != nil {
		shell.printError(err.Error(), true)
	}

	// shell.start()

	go func() {
		<-quitChan
		fmt.Printf("received stop exiting")
		conn.Quit()
		os.Exit(0)
	}()

	loop := true

	// doneChanStr := make(chan []string, 10)
	doneChanStruct := make(chan struct{}, 10)
	errChan := make(chan error, 10)
	abortChan := make(chan struct{}, 10)
	startingChan := make(chan struct{}, 10)
	// onEachChan := make(chan struct{}, 10)

	var gotResponse interface{}

	fmt.Printf("Server tells: %s\n", response.String())

	var location string
	if alwaysPwd {
		location = localIPParsed.String() + "/"
	} else {
		location = localIPParsed.String()
	}

	// skipNextScanLine := false
	prompt := true
	var lockSkipNextScanLine sync.Mutex

	// unlocked := false
	for loop {

		lockSkipNextScanLine.Lock()

		// if skipNextScanLine {
		// 	skipNextScanLine = false
		// 	// shell.scanLine()
		// 	// shell.scanLine()
		// 	shell.flush()
		// 	lockSkipNextScanLine.Unlock()
		// 	unlocked = true
		// 	continue
		// }
		// if unlocked == false {
		// 	unlocked = true
		// 	lockSkipNextScanLine.Unlock()
		// }
		if prompt {
			shell.prompt(location)
		} else {
			prompt = true
		}
		lockSkipNextScanLine.Unlock()
		line := shell.scanLine()

		// lockSkipNextScanLine.Unlock()

		// shell.print(fmt.Sprintf("I read '%s'", line))

		if strings.HasPrefix(line, "Operation ") ||
			strings.HasPrefix(line, "\nOperation") ||
			strings.HasPrefix(line, "\n") ||
			line == "" {
			if prompt == false {
				prompt = true
			}
			continue
		}
		// lockSkipNextScanLine.Unlock()
		// splitting line
		// splitted_line := strings.Split(line, " ")
		// if len(splitted_line) >

		if line == quit {
			if _, err := conn.Quit(); err != nil {
				shell.printError(err.Error(), false)
			}
			shell.goodbye()
			loop = false

		} else if strings.HasPrefix(line, help) {
			helpCmd := strings.Split(line, " ")
			if len(helpCmd) == 1 {
				for key, value := range helpMap {
					shell.print(key + ":\t\t\t" + value.String(false) + "\n")
				}
			} else {
				helpMsg, ok := helpMap[helpCmd[1]]
				if !ok {
					shell.print(unrecognizedCmd + "\n")
				} else {
					shell.print(helpMsg.String(true) + "\n")
				}
			}
		} else {
			cmd, err := parse(line)
			if err != nil {
				shell.printError(err.Error(), false)
				continue
			}
			doneChan := doneChanStruct
			// var doneChan interface{}
			if cmd.cmd == ls {
				dirs, err := cmd.apply(conn, true)
				if err != nil {
					shell.printError(err.Error(), false)
				} else {
					for _, dir := range dirs.([]string) {
						shell.print(dir)
					}
				}

			} else if cmd.cmd == put || cmd.cmd == get {

				onEachChan := make(chan struct{}, 1)

				// issuing a size
				_, size, err := conn.Size(cmd.args[0])
				if err != nil {
					shell.printError(err.Error(), false)
					continue
				}

				var pb *pb.ProgressBar

				if asyncDownload == false {
					pb = shell.displayProgressBar(size)
					pb.Start()
				} else {
					onEachChan = nil
				}

				cmd.apply(conn, false, doneChanStruct, errChan, abortChan,
					startingChan,
					onEachChan)

				<-startingChan

				if asyncDownload {
					go func() {
						<-doneChanStruct
						lockSkipNextScanLine.Lock()
						shell.print("\n")
						shell.print(fmt.Sprintf("Operation %s on %v finished\n", cmd.cmd, cmd.args))
						shell.prompt(location)
						prompt = false
						lockSkipNextScanLine.Unlock()
					}()
				} else {

					for _ = range onEachChan {
						pb.Increment()
					}
					<-doneChanStruct
					pb.Finish()
					pb.FinishPrint("Operation completed")
				}

			} else {
				gotResponse, err = cmd.apply(conn, true, doneChan, errChan, abortChan, startingChan)
				if err != nil {
					shell.printError(err.Error(), false)
					continue
				}
				if gotResponse != nil {
					switch gotResponse.(type) {
					case string:
						shell.print(gotResponse.(string))
					case []interface{}:
						shell.print(gotResponse.([]interface{})[0].(string))
					case *ftp.Response:
						shell.print(gotResponse.(*ftp.Response).String())
						// shell.print(response.String())
					}
				}
			}

			if cmd.cmd == "cd" && alwaysPwd {
				currentDir, err := commandPwd.apply(conn, true)
				if err != nil {
					// do nothing, it's a command that hasn't been required by the user.
				} else {
					currentDir := currentDir.(string)
					location = localIPParsed.String() + currentDir
				}
			}

		}
	}

	defer conn.Quit()

}
