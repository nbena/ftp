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
	return ftp.DialAndAuthenticate(remote,
		&ftp.Config{
			TLSOption: &ftp.TLSOption{
				AllowSSL:        allowSSL3,
				SkipVerify:      skipVerify,
				AuthTLSOnFirst:  authTLSOnFirst,
				ContinueIfNoSSL: continueIfNoTLS,
				ImplicitTLS:     implicitTLS,
			},
			DefaultMode: ftp.FTP_MODE_IND,
			LocalIP:     localIPParsed,
			LocalPort:   localPortParsed,
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
			if v.cmd == CommandQuit.cmd {
				// execute and exit.

			}
		}
	}

	shell := newshell()
	shell.askCredential()

	conn, _, err := getConn()
	if err != nil {
		shell.printError(err.Error(), true)
	}

	loop := true
	for loop {
		shell.prompt()
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
					shell.print(key + ":\t" + value)
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
			cmd.apply(nil)
		}
	}

	defer conn.Quit()

}
