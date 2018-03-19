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
	"fmt"
	"os"

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
		fmt.Printf("Ciphers: \n")
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

	defer conn.Quit()

}
