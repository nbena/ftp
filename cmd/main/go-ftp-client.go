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
	interactiveMode := true

	quitChan := make(chan os.Signal)
	signal.Notify(quitChan, syscall.SIGINT, syscall.SIGSTOP)

	if showCiphers {
		ciphers := ftp.CipherSuitesString(allowWeakHash)
		for _, cipher := range ciphers {
			fmt.Printf("%s\n", cipher)
		}
		os.Exit(0)
	}

	// if len(parsedCommands) > 0 {
	// 	interactiveMode = false
	// 	for _, v := range parsedCommands {
	// 		if v.cmd == commandQuit.cmd {
	// 			// execute and exit.
	// 		}
	// 	}
	// }
	if len(commandsArray) > 0 {
		interactiveMode = false
	}

	shell := newShell()
	if username == "" || password == "" && interactiveMode {
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

	// fmt.Printf("Server tells: %s\n", response.String())
	shell.print(fmt.Sprintf("Server tells: %s\n", response.String()))

	var location string
	if alwaysPwd {
		location = localIPParsed.String() + "/"
	} else {
		location = localIPParsed.String()
	}

	// skipNextScanLine := false
	prompt := true
	var lockSkipNextScanLine sync.Mutex
	currentCommandsIndex := 0

	// unlocked := false
	for loop {

		lockSkipNextScanLine.Lock()

		if prompt && interactiveMode {
			shell.prompt(location)
		} else {
			prompt = true
		}
		lockSkipNextScanLine.Unlock()

		var line string

		if interactiveMode {
			line = shell.scanLine()
		} else {
			line = commandsArray[currentCommandsIndex]
		}

		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "Operation ") ||
			strings.HasPrefix(line, "\nOperation") ||
			strings.HasPrefix(line, "\n") ||
			line == "" {
			if prompt == false {
				prompt = true
			}
			if !interactiveMode {
				currentCommandsIndex++
			}
			continue
		}

		if currentCommandsIndex == len(commandsArray)-1 && !interactiveMode {
			loop = false
			// next iteration we'll exit.
		}
		currentCommandsIndex++

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
				// dirs declared to prevent err to be shadowed.
				var dirs interface{}
				dirs, err = cmd.apply(conn, true)
				if err != nil {
					shell.printError(err.Error(), false)
				} else {
					for _, dir := range dirs.([]string) {
						shell.print(dir)
					}
				}

			} else if cmd.cmd == put || cmd.cmd == get {

				onEachChan := make(chan struct{}, 1)

				// issuing a remote size only if download
				var size int
				if cmd.cmd == get {
					_, size, err = conn.Size(cmd.args[0])
					if err != nil {
						shell.printError(err.Error(), false)
						continue
					}
				} else {
					// if it is a put we get the size from the
					// local file.
					var file *os.File
					var info os.FileInfo
					file, err = os.Open(cmd.args[0])
					if err != nil {
						shell.printError(err.Error(), false)
					}
					info, err = file.Stat()
					if err != nil {
						shell.printError(err.Error(), false)
					}
					size = int(info.Size())
					err = file.Close()
					if err != nil {
						shell.printError(err.Error(), false)
					}
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
					last := 0
					for _ = range onEachChan {
						// pb.Increment()
						last += conn.BufferSize()
						set := last
						if last > size {
							set = size
						}

						pb.Set(set)
					}
					<-doneChanStruct

					pb.Set(size)
					pb.Finish()
					pb.FinishPrint(fmt.Sprintf("Operation %s on %v finished\n", cmd.cmd, cmd.args))
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
						shell.print(gotResponse.(string) + "\n")
					case []interface{}:
						shell.print(gotResponse.([]interface{})[0].(string) + "\n")
					case *ftp.Response:
						shell.print(gotResponse.(*ftp.Response).String() + "\n")
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

	// defer conn.Quit()

}
