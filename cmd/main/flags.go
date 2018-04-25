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
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/nbena/ftp"
)

var (
	localIP         string
	localIPParsed   net.IP
	localPortParsed int
	remote          string
	defaultMode     string
	username        string
	password        string
	implicitTLS     bool
	authTLSOnFirst  bool
	allowSSL3       bool
	continueIfNoTLS bool
	allowWeakHash   bool
	skipVerify      bool
	serverName      string
	showCiphers     bool
	commands        string
	parsedCommands  []*cmd
	deleteIfAbort   bool
	alwaysPwd       bool
	asyncDownload   bool

	ftpDefaultMode ftp.Mode

	commandsArray []string
)

func parseFlags() {
	flag.StringVar(&localIP, "local-address", "localhost:5354", "the address:port which the client binds in")
	flag.StringVar(&remote, "remote", "localhost:2121", "name:port of ftp server")
	flag.StringVar(&defaultMode, "connection-mode", "passive", "the ftp mode, allowed: passive|active|default")
	flag.StringVar(&username, "username", "anonymous", "the username")
	flag.StringVar(&password, "password", "c@b.com", "the password")
	flag.BoolVar(&implicitTLS, "tls-implicit", false, "use implicit TLS")
	flag.BoolVar(&authTLSOnFirst, "tls-auth-first", true, "run auth TLS asap")
	flag.BoolVar(&allowSSL3, "tls-allow-ssl3", false, "allow or not SSL3")
	flag.BoolVar(&continueIfNoTLS, "tls-continue-if-no", false, "continue if TLS doesn't work")
	flag.BoolVar(&allowWeakHash, "tls-allow-sha", false, "allow ciphers with SHA hash")
	flag.BoolVar(&skipVerify, "tls-skip-verify", false, "skip or not the server cert verification")
	flag.BoolVar(&showCiphers, "tls-show-ciphers", false, "show available TLS ciphers")
	flag.StringVar(&commands, "commands", "", "list of semicolon-separated commands to be executed")
	// flag.StringVar(&anonymous, "anonymous-ftp", true, "use anonym")
	flag.BoolVar(&alwaysPwd, "always-run-pwd", true, "after every CD run an LS too show the current directory in prompt")
	flag.BoolVar(&asyncDownload, "async-download", true, "when down/uploading a file, use a background transfering")

	flag.Parse()

	// now checking
	var err error

	if defaultMode != "passive" && defaultMode != "active" && defaultMode != "default" {
		fmt.Fprintf(os.Stderr, "Unknow option for \"connection-mode\": %s", defaultMode)
		os.Exit(1)
	}

	ftpDefaultMode, err = ftp.GetMode(defaultMode)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
	if ftpDefaultMode == ftp.IndMode {
		ftpDefaultMode = ftp.PassiveMode
	}

	// if commands != "" {
	// 	parsedCommands, err = parseAllCommands(commands)
	// 	if err != nil {
	// 		fmt.Fprintf(os.Stderr, err.Error())
	// 		os.Exit(1)
	// 	}
	// }
	if commands != "" {
		commandsArray = strings.Split(commands, ";")
		if commandsArray[len(commandsArray)-1] != quit {
			// if a quit is not provided we add by ourselves(?) the command
			commandsArray = append(commandsArray, quit)
		}
	}

	splittedIP := strings.Split(localIP, ":")
	if len(splittedIP) != 2 {
		fmt.Fprintf(os.Stderr, "Invalid format for local-address: %s", localIP)
		os.Exit(1)
	}

	localPortParsed, err = strconv.Atoi(splittedIP[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid format for local-address: %s", localIP)
		os.Exit(1)
	}

	if splittedIP[0] == "localhost" {
		splittedIP[0] = "127.0.0.1"
	}

	localIPParsed = net.ParseIP(splittedIP[0])
	if localIPParsed == nil {
		fmt.Fprintf(os.Stderr, "Invalid format for local-address: %s", localIP)
		os.Exit(1)
	}
}
