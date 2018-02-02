package main

import (
	"crypto/tls"
	"flag"
	"net"
	"os"

	"strconv"

	"github.com/nbena/go-ftp"
	// shellLib "github.com/nbena/go-ftp/shell"
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

	args, port, options := parseArgv()

	uri := args[0] + ":" + strconv.Itoa(port)

	//shell := shellLib.NewShell()
	shell := NewShell()

	//username, password := shell.AskCredential()
	username, password := args[1], args[2]

	var printFile *os.File

	if options[0] {
		printFile = os.Stdout
	} else {
		printFile, _ = os.Open(os.DevNull)
	}

	var tlsConfig *tls.Config = nil

	if options[1] {
		tlsConfig = &tls.Config{
			InsecureSkipVerify:       true,
			PreferServerCipherSuites: true}
	}

	// var mode ftp.Mode
	//
	// if args[3] == "active" {
	// 	mode = ftp.FTP_ACTIVE
	// } else if args[3] == "passive" {
	// 	mode = ftp.FTP_PASSIVE
	// }

	params := &ftp.Params{
		DefaultMode:  ftp.Mode(args[3]),
		ResponseFile: printFile,
		TLSConfig:    tlsConfig}

	/*conn, err := */
	shell.LogAndAuth(uri)

	ftpConn, err := ftp.ConnectTo(uri, params)
	if err != nil {
		panic(err)
	}

	//stdin := bufio.NewReader(os.Stdin)

	if _, err = ftpConn.Authenticate(username, password); err != nil {
		panic(err)
	}

	shell.Print("Successfully authenticated")

	response, err := ftpConn.Port(net.IPv4(127, 0, 0, 1), 1930)
	if err != nil {
		panic(err)
	}
	shell.Print(response.String())

	ftpConn.Quit()

}

// func main() {
// 	con, _ := net.Dial("tcp", "localhost:2121")
// 	buf := make([]byte, 1024)
// 	con.Read(buf)
// 	con.Write([]byte("USER anonymous\r\n"))
// 	con.Read(buf)
// 	con.Write([]byte("PASS password\r\n"))
// 	buf2 := make([]byte, 1024)
// 	con.Read(buf2)
// 	fmt.Printf("%s", string(buf2))
// }
