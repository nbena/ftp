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
	"flag"
)

func parseArgv() ([]string, int, []bool) {
	host := flag.String("host", "localhost", "the name of the ftp server")
	port := flag.Int("port", 21, "the port to connect to")
	defaultMode := flag.String("mode", "active", "the ftp modality")
	printRes := flag.Bool("print-raw-response", false, "print or no the server responses")
	username := flag.String("username", "anonymous", "the username")
	password := flag.String("password", "c@b.com", "the password")
	tls := flag.Bool("tls", true, "use or not a tunnel over TLS")
	flag.Parse()
	argv := []string{*host, *username, *password, *defaultMode}
	return argv, *port, []bool{*printRes, *tls}
}

func main() {

}
