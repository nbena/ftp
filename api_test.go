package ftp

import (
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"testing"
)

func TestDial(t *testing.T) {

	ftpConn, resp, err := Dial("localhost:2121", &Config{
		DefaultMode: FTP_MODE_ACTIVE,
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
		DefaultMode: FTP_MODE_ACTIVE,
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
		DefaultMode: FTP_MODE_ACTIVE,
		Username:    "anonymous",
		Password:    "c@b.com",
		LocalIP:     net.IP([]byte{127, 0, 0, 1}),
	})

	defer ftpConn.Quit()

	if err != nil {
		t.Errorf("Error in conn: %s", err.Error())
	}

	// t.Log(resp.String())

	resp, port, err := ftpConn.port()
	if err != nil {
		t.Errorf("Error in port: %s", err.Error())
		return
	}

	t.Logf("PORT resp: %s", resp.String())
	t.Logf("Going to port at: %d", port)

}

func TestIsPortAvailable(t *testing.T) {

	available := isPortAvailable(80)
	if available == true {
		t.Fatalf("Port shouldn't be available")
	}

}

func TestStoreRetrDeleteFile(t *testing.T) {

	ftpConn, _, err := DialAndAuthenticate("localhost:2121", &Config{
		DefaultMode: FTP_MODE_ACTIVE,
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

	doneChanStore := make(chan struct{})
	errChanStore := make(chan error, 1)

	go ftpConn.Store("tmp.txt", FTP_MODE_ACTIVE, doneChanStore, errChanStore)

	select {
	case err = <-errChanStore:
		t.Errorf("Got error: %s", err.Error())
		return
	case <-doneChanStore:
	}

	doneChanRetr := make(chan struct{})
	errChanRetr := make(chan error, 1)
	go ftpConn.Retrieve(FTP_MODE_ACTIVE, "tmp.txt", "temp_get.txt", doneChanRetr, errChanRetr)

	select {
	case err = <-errChanRetr:
		t.Errorf("Got error: %s", err.Error())
		return
	case <-doneChanRetr:

		//reading downloaded file.
		var content []byte // just to prevent go vet
		content, err = ioutil.ReadFile("temp_get.txt")
		if err != nil {
			t.Errorf("Got error: %s", err.Error())
			return
		}
		if !reflect.DeepEqual(fileContent, content) {
			t.Errorf("Mismatched files")
			t.Errorf("string content:\n%s\n%s", string(fileContent), string(content))
			t.Logf("byte content:\n%v\n%v", fileContent, content)
			return
		}
	}

	response, err := ftpConn.DeleteFile("tmp.txt")
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
		return
	}

	t.Logf("DELE resp: %s", response.String())

}

func TestParsePasvOk(t *testing.T) {

	response := newFtpResponse("227 Entering Passive Mode (127,0,0,1,179,36)")
	addr, err := parsePasv(response)
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
		return
	}
	t.Logf("TCPAddr is: %s", addr.String())
}

func TestDirOps(t *testing.T) {

	ftpConn, _, err := DialAndAuthenticate("localhost:2121", &Config{
		DefaultMode: FTP_MODE_ACTIVE,
		Username:    "anonymous",
		Password:    "c@b.com",
		LocalIP:     net.IP([]byte{127, 0, 0, 1}),
	})

	defer ftpConn.Quit()

	if err != nil {
		t.Errorf("Got error: %s", err.Error())
		return
	}

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

	doneChanLs := make(chan []string)
	errChanLs := make(chan error, 1)
	var lsResponse []string

	var lsDirResponse []string
	doneChanDir := make(chan []string)
	errChanDir := make(chan error, 1)

	go ftpConn.Ls(FTP_MODE_ACTIVE, doneChanLs, errChanLs)
	select {
	case lsResponse = <-doneChanLs:
	case err = <-errChanLs:
		t.Errorf("Got error: %s", err.Error())
		return
	}

	go ftpConn.LsDir(FTP_MODE_ACTIVE, "temp", doneChanDir, errChanDir)
	select {
	case lsDirResponse = <-doneChanDir:
	case err = <-errChanDir:
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
	t.Logf("LIST resp:\n%v", lsResponse)
	t.Logf("LIST temp resp:\n%v", lsDirResponse)
	t.Logf("RMD temp resp: %s", rmResponse.String())

}

// func TestLs(t *testing.T) {
//
// 	ftpConn, _, err := DialAndAuthenticate("localhost:2121", &Config{
// 		DefaultMode: FTP_MODE_ACTIVE,
// 		Username:    "anonymous",
// 		Password:    "c@b.com",
// 		LocalIP:     net.IP([]byte{127, 0, 0, 1}),
// 	})
//
// 	defer ftpConn.Quit()
//
// 	if err != nil {
// 		t.Errorf("Got error: %s", err.Error())
// 		return
// 	}
//
// 	doneChan := make(chan []string)
// 	errChan := make(chan error, 1)
//
// 	go ftpConn.Ls(FTP_MODE_ACTIVE, doneChan, errChan)
//
// 	// for {
// 	select {
// 	case err := <-errChan:
// 		t.Errorf("Got error: %s", err.Error())
// 		return
// 	case result := <-doneChan:
// 		// for _, v := range result {
// 		// 	fmt.Printf("%s\n", v)
// 		// }
// 		t.Logf("LIST is:\n%+v", result)
// 		return
// 		// default:
// 		// 	fmt.Printf("i'm waiting")
// 	}
// 	// }
//
// }
