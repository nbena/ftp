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
