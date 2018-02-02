package ftp

import (
	"net"
	"os"
	"testing"
)

func TestDial(t *testing.T) {

	ftpConn, resp, err := Dial("localhost:2121", &Config{
		DefaultMode: FTP_ACTIVE,
		Username:    "anonymous",
		Password:    "c@b.com",
		LocalIP:     net.IP([]byte{127, 0, 0, 1}),
	})

	defer ftpConn.Quit()

	if err != nil {
		t.Errorf("Error in conn: %s", err.Error())
	}

	t.Log(resp.String())

	resp, err = ftpConn.Authenticate()
	if err != nil {
		t.Errorf("Error in auth: %s", err.Error())
	}

	t.Log(resp.String())

}

func TestDialAndAuthenticate(t *testing.T) {

	ftpConn, resp, err := DialAndAuthenticate("localhost:2121", &Config{
		DefaultMode: FTP_ACTIVE,
		Username:    "anonymous",
		Password:    "c@b.com",
		LocalIP:     net.IP([]byte{127, 0, 0, 1}),
	})

	defer ftpConn.Quit()

	if err != nil {
		t.Errorf("Error in conn: %s", err.Error())
	}

	t.Log(resp.String())

}

// func TestGenPort(t *testing.T) {
//
// 	ftpConn := &Conn{lastUsedPort: 1930}
// 	port, n1, n2 := ftpConn.getRandomPort()
// 	t.Log(port, n1, n2)
// }

func TestPort(t *testing.T) {

	ftpConn, resp, err := DialAndAuthenticate("localhost:2121", &Config{
		DefaultMode: FTP_ACTIVE,
		Username:    "anonymous",
		Password:    "c@b.com",
		LocalIP:     net.IP([]byte{127, 0, 0, 1}),
	})

	defer ftpConn.Quit()

	if err != nil {
		t.Errorf("Error in conn: %s", err.Error())
	}

	t.Log(resp.String())

	resp, err = ftpConn.Port()
	if err != nil {
		t.Errorf("Error in port: %s", err.Error())
	}

	t.Log(resp.String())

}

func TestIsPortAvailable(t *testing.T) {

	available := isPortAvailable(80)
	if available == true {
		t.Fatalf("Port shouldn't be available")
	}

}

func TestPutGo(t *testing.T) {

	ftpConn, resp, err := DialAndAuthenticate("localhost:2121", &Config{
		DefaultMode: FTP_ACTIVE,
		Username:    "anonymous",
		Password:    "c@b.com",
		LocalIP:     net.IP([]byte{127, 0, 0, 1}),
	})

	defer ftpConn.Quit()

	if err != nil {
		t.Errorf("Error in conn: %s", err.Error())
	}

	t.Log(resp.String())

	fileContent := []byte("hello this is an example")
	file, err := os.Create("tmp.txt")
	defer os.Remove("tmp.txt")
	if err != nil {
		t.Errorf(err.Error())
		return
	}
	_, err = file.Write(fileContent)
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	done := make(chan string, 1)

	ftpConn.PutGo("tmp.txt", nil, done)
	ret := <-done
	if ret != "" {
		t.Errorf("Response: %s", ret)
	}

}
