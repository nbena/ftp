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

package ftp

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

func portString(ip net.IP, n1, n2 int) string {
	return strings.Replace(ip.String(), ".", ",", 4) + "," + strconv.Itoa(n1) + "," + strconv.Itoa(n2)
}

// func portNumbers(port int) (int, int) {
// 	n1 := port / 256
// 	n2 := port - (n1 * 256)
// 	return n1, n2
// }

func (f *Conn) getRandomPort() (port, n1, n2 int) {

	f.portLock.Lock()
	defer f.portLock.Unlock()

	port = f.lastUsedPort + 1

	for !isPortAvailable(port) {
		port++
	}

	f.lastUsedPort = port

	n1 = port / 256
	n2 = port - (n1 * 256)
	// log.Printf("Port is: %d", port)
	return port, n1, n2
}

func isPortAvailable(port int) bool {
	available := true
	conn, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		available = false
	} else {
		conn.Close()
	}
	return available
}
