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
	"strings"

	"github.com/nbena/ftp"
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

	shell := newshell()
	if username == "" || password == "" {
		username, password = shell.askCredential()
	}
	conn, response, err := getConn()
	if err != nil {
		shell.printError(err.Error(), true)
	}

	loop := true

	doneChanStr := make(chan []string, 10)
	doneChanStruct := make(chan struct{}, 10)
	errChan := make(chan error, 10)
	abortChan := make(chan struct{}, 10)
	startingChan := make(chan struct{}, 10)
	onEachChan := make(chan struct{}, 10)

	var gotResponse interface{}

	fmt.Printf("Server tells: %s\n", response.String())

	var location string
	if alwaysPwd {
		location = localIPParsed.String() + "/"
	} else {
		location = localIPParsed.String()
	}

	for loop {
		shell.prompt(location)
		line := shell.scanLine()
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
					shell.print(key + ":\t\t" + value)
				}
			} else {
				helpMsg, ok := helpMap[helpCmd[1]]
				if !ok {
					shell.print(unrecognizedCmd)
				} else {
					shell.print(helpMsg)
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
				/*_, err = */ cmd.apply(conn, doneChanStr, errChan, abortChan, startingChan)

				select {
				case <-errChan:
					shell.printError(err.Error(), false)

				case dirs := <-doneChanStr:
					for _, dir := range dirs {
						shell.print(dir)
					}
				}
				continue

			} else if cmd.cmd == put || cmd.cmd == get {

				// issuing a size
				_, size, err := conn.Size(cmd.args[0])
				if err != nil {
					shell.printError(err.Error(), false)
					continue
				}

				pb := shell.displayProgressBar(size)

				// doneChan = doneChanStruct
				_, err = cmd.apply(conn, doneChanStruct, errChan, abortChan,
					startingChan,
					onEachChan)

				if err != nil {
					go func() {
						select {
						case <-doneChanStruct:
							pb.Finish()
						case <-onEachChan:
							pb.Increment()
						case <-startingChan:
							pb.Start()
						}
					}()
				} else {
					shell.printError(err.Error(), false)
					continue
				}

			} else {
				gotResponse, err = cmd.apply(conn, doneChan, errChan, abortChan, startingChan)
				if err != nil {
					shell.printError(err.Error(), false)
					continue
				}
			}

			// _, err = cmd.apply(conn, doneChan, errChan, abortChan, startingChan)
			// if err != nil {
			// 	shell.printError(err.Error(), false)
			// 	continue
			// }

			// if cmd.cmd == ls {
			// 	dirs := <-doneChanStr
			// 	for _, dir := range dirs {
			// 		shell.print(dir)
			// 	}
			// }
			if gotResponse != nil {
				switch gotResponse.(type) {
				case string:
					shell.print(gotResponse.(string))
				}
				// shell.print(response.String())
			}

			if cmd.cmd == "cd" && alwaysPwd {
				currentDir, err := commandPwd.apply(conn)
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
