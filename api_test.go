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
		return
	}

	t.Log(resp.String())

	resp, err = ftpConn.Authenticate()
	if err != nil {
		t.Errorf("Error in auth: %s", err.Error())
		return
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

	// t.Log(resp.String())

	resp, err = ftpConn.Port()
	if err != nil {
		t.Errorf("Error in port: %s", err.Error())
		return
	}

	t.Logf("PORT resp: %s", resp.String())

}

func TestIsPortAvailable(t *testing.T) {

	available := isPortAvailable(80)
	if available == true {
		t.Fatalf("Port shouldn't be available")
	}

}

func TestStoreAndDeleteFile(t *testing.T) {

	ftpConn, _, err := DialAndAuthenticate("localhost:2121", &Config{
		DefaultMode: FTP_ACTIVE,
		Username:    "anonymous",
		Password:    "c@b.com",
		LocalIP:     net.IP([]byte{127, 0, 0, 1}),
	})

	defer ftpConn.Quit()

	if err != nil {
		t.Errorf("Error in conn: %s", err.Error())
		return
	}

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

	// done := make(chan string, 1)
	//
	// ftpConn.PutGo("tmp.txt", nil, done)
	// ret := <-done
	// if ret != "" {
	// 	t.Errorf("Response: %s", ret)
	// }

	done := make(chan string, 1)
	ftpConn.Store("tmp.txt", nil, done)
	ret := <-done
	if ret != "" {
		t.Errorf("Response: %s", ret)
		return
	}

	response, err := ftpConn.DeleteFile("tmp.txt")
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
		return
	}

	t.Logf("DELE resp: %s", response.String())

}

func TestMkAndCdAndRmdir(t *testing.T) {

	ftpConn, _, err := DialAndAuthenticate("localhost:2121", &Config{
		DefaultMode: FTP_ACTIVE,
		Username:    "anonymous",
		Password:    "c@b.com",
		LocalIP:     net.IP([]byte{127, 0, 0, 1}),
	})

	defer ftpConn.Quit()

	mkResponse, err := ftpConn.MkDir("temp")
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
		return
	}

	// cd-ing into temp.
	cdInResponse, err := ftpConn.Cd("temp")
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
		return
	}

	cdOutResponse, err := ftpConn.Cd("..")
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
		return
	}

	rmResponse, err := ftpConn.DeleteDir("temp")
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
		return
	}

	t.Logf("MKD temp resp: %s", mkResponse.String())
	t.Logf("CWD temp resp: %s", cdInResponse.String())
	t.Logf("CWD .. resp: %s", cdOutResponse.String())
	t.Logf("RMD temp resp: %s", rmResponse.String())

}
