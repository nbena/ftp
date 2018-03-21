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
