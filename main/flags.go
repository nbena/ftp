package main

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

import (
	"flag"
	"fmt"
	"os"
)

var (
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
	showCiphers     bool
	commands        string
	parsedCommands  []*cmd
)

func parseFlags() {
	flag.StringVar(&remote, "remote", "localhost:2121", "name:port of ftp server")
	flag.StringVar(&defaultMode, "connection-mode", "active", "the ftp mode, allowed: passive|active|default")
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

	flag.Parse()

	// now checking
	var err error

	if defaultMode != "passive" && defaultMode != "active" && defaultMode != "default" {
		fmt.Fprintf(os.Stderr, "Unknow option for \"connection-mode\": %s", defaultMode)
		os.Exit(1)
	}

	if commands != "" {
		parsedCommands, err = parseAllCommands(commands)
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
	}
}
