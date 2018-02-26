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

package ftp

import (
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"testing"
)

func authenticatedConn() (*Conn, *Response, error) {
	return DialAndAuthenticate("localhost:2121", &Config{
		DefaultMode: FTP_MODE_ACTIVE,
		Username:    "anonymous",
		Password:    "c@b.com",
		LocalIP:     net.IP([]byte{127, 0, 0, 1}),
	})
}

func TestDial(t *testing.T) {

	ftpConn, resp, err := Dial("localhost:2121", &Config{
		DefaultMode: FTP_MODE_ACTIVE,
		Username:    "anonymous",
		Password:    "c@b.com",
		LocalIP:     net.IP([]byte{127, 0, 0, 1}),
	})

	if err != nil {
		t.Errorf("Error in conn: %s", err.Error())
		return
	}

	defer ftpConn.Quit()

	t.Log(resp.String())

	resp, err = ftpConn.Authenticate()
	if err != nil {
		t.Errorf("Error in auth: %s", err.Error())
		return
	}

	t.Log(resp.String())

}

func TestDialAndAuthenticate(t *testing.T) {

	ftpConn, resp, err := authenticatedConn()

	if err != nil {
		t.Fatalf("Conn error: %s", err.Error())
	}

	defer ftpConn.Quit()

	t.Logf(resp.String())

}

func TestPort(t *testing.T) {

	ftpConn, resp, err := authenticatedConn()

	if err != nil {
		t.Fatalf("Conn error: %s", err.Error())
	}

	defer ftpConn.Quit()

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

func TestParsePasvOk(t *testing.T) {

	response, err := newFtpResponse("227 Entering Passive Mode (127,0,0,1,179,36)")
	if err != nil {
		t.Fatalf("Got error: %s", err.Error())
	}
	addr, err := parsePasv(response)
	if err != nil {
		t.Fatalf("Got error: %s", err.Error())
	}
	t.Logf("TCPAddr is: %s", addr.String())
	if addr.Port != 45860 {
		t.Fatalf("Port is not correct")
	}
}

func TestFileOpsActive(t *testing.T) {

	ftpConn, _, err := authenticatedConn()

	if err != nil {
		t.Fatalf("Conn error: %s", err.Error())
	}

	defer ftpConn.Quit()

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

	doneChanStore := make(chan struct{}, 1)
	abortChanStore := make(chan struct{}, 1)
	errChanStore := make(chan error, 1)

	go ftpConn.Store("tmp.txt", FTP_MODE_ACTIVE, doneChanStore, abortChanStore, errChanStore)

	select {
	case err = <-errChanStore:
		t.Errorf("Got error: %s", err.Error())
		return
	case <-doneChanStore:
	}

	doneChanRetr := make(chan struct{}, 1)
	abortChanRetr := make(chan struct{}, 1)
	errChanRetr := make(chan error, 1)
	go ftpConn.Retrieve(FTP_MODE_ACTIVE, "tmp.txt", "temp_get.txt", doneChanRetr, abortChanRetr, errChanRetr)

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

func TestFileOpsPassive(t *testing.T) {

	ftpConn, _, err := authenticatedConn()

	if err != nil {
		t.Fatalf("Conn error: %s", err.Error())
	}

	defer ftpConn.Quit()

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

	doneChanStore := make(chan struct{})
	abortChanStore := make(chan struct{}, 1)
	errChanStore := make(chan error, 1)

	go ftpConn.Store("tmp.txt", FTP_MODE_PASSIVE, doneChanStore, abortChanStore, errChanStore)

	select {
	case err = <-errChanStore:
		t.Errorf("Got error: %s", err.Error())
		return
	case <-doneChanStore:
	}

	doneChanRetr := make(chan struct{}, 1)
	abortChanRetr := make(chan struct{}, 1)
	errChanRetr := make(chan error, 1)

	go ftpConn.Retrieve(FTP_MODE_PASSIVE, "tmp.txt", "temp_get.txt", doneChanRetr, abortChanRetr, errChanRetr)

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

func TestDirOpsActive(t *testing.T) {

	ftpConn, _, err := authenticatedConn()

	if err != nil {
		t.Fatalf("Conn error: %s", err.Error())
	}

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

	pwdResponse, directory, err := ftpConn.Pwd()
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
		return
	}
	if directory != "/temp" {
		t.Errorf("Error: want %s, got %s", "temp", directory)
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

	// this will throw an error

	doneChanLsErr := make(chan []string)
	errChanLsErr := make(chan error, 1)

	go ftpConn.LsDir(FTP_MODE_ACTIVE, "tmp_renamed", doneChanLsErr, errChanLsErr)

	select {
	case err = <-errChanLsErr:
		//if !inverseResponse(err.Error()).IsFileNotExists() {
		// building a response from this error.
		response, err := newFtpResponse(err.Error())
		if err != nil {
			t.Errorf(err.Error())
			return
		}
		if !response.IsFileNotExists() {
			t.Errorf("Got error: %s", err.Error())
			return
		}
	case <-doneChanLsErr:
		t.Errorf("Got 0 error while expecting one")
		return
	}

	rmResponse, err := ftpConn.DeleteDir("temp")
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
		return
	}

	t.Logf("MKD temp resp: %s", mkResponse.String())
	t.Logf("CWD temp resp: %s", cdInResponse.String())
	t.Logf("PWD resp: %s", pwdResponse.String())
	t.Logf("CWD .. resp: %s", cdOutResponse.String())
	t.Logf("LIST resp:\n%v", lsResponse)
	t.Logf("LIST temp resp:\n%v", lsDirResponse)
	t.Logf("RMD temp resp: %s", rmResponse.String())

}

func TestDirOpsPassive(t *testing.T) {

	ftpConn, _, err := authenticatedConn()

	if err != nil {
		t.Fatalf("Conn error: %s", err.Error())
	}

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

	pwdResponse, directory, err := ftpConn.Pwd()
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
		return
	}
	if directory != "/temp" {
		t.Errorf("Error: want %s, got %s", "temp", directory)
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

	// this will throw an error

	doneChanLsErr := make(chan []string)
	errChanLsErr := make(chan error, 1)

	go ftpConn.LsDir(FTP_MODE_ACTIVE, "tmp_renamed", doneChanLsErr, errChanLsErr)

	select {
	case err = <-errChanLsErr:
		response, err := newFtpResponse(err.Error())
		if err != nil {
			t.Errorf(err.Error())
			return
		}
		if !response.IsFileNotExists() {
			t.Errorf("Got error: %s", err.Error())
			return
		}
	case <-doneChanLsErr:
		t.Errorf("Got 0 error while expecting one")
		return
	}

	rmResponse, err := ftpConn.DeleteDir("temp")
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
		return
	}

	t.Logf("MKD temp resp: %s", mkResponse.String())
	t.Logf("CWD temp resp: %s", cdInResponse.String())
	t.Logf("PWD resp: %s", pwdResponse.String())
	t.Logf("CWD .. resp: %s", cdOutResponse.String())
	t.Logf("LIST resp:\n%v", lsResponse)
	t.Logf("LIST temp resp:\n%v", lsDirResponse)
	t.Logf("RMD temp resp: %s", rmResponse.String())

}

func TestRename(t *testing.T) {

	ftpConn, _, err := authenticatedConn()

	if err != nil {
		t.Fatalf("Conn error: %s", err.Error())
	}

	defer ftpConn.Quit()

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

	doneChanStore := make(chan struct{})
	abortChanStore := make(chan struct{}, 1)
	errChanStore := make(chan error, 1)

	go ftpConn.Store("tmp.txt", FTP_MODE_PASSIVE, doneChanStore, abortChanStore, errChanStore)

	select {
	case err = <-errChanStore:
		t.Errorf("Got store error: %s", err.Error())
		return
	case <-doneChanStore:
	}

	t.Logf("stored\n")

	responseRename, err := ftpConn.Rename("tmp.txt", "tmp_renamed.txt")
	if err != nil {
		t.Errorf("Got rename error: %s", err.Error())
		return
	}

	t.Logf("I've done the renaming")

	doneChanLs := make(chan []string)
	errChanLs := make(chan error, 1)
	var ls []string

	go ftpConn.LsDir(FTP_MODE_ACTIVE, "tmp_renamed.txt", doneChanLs, errChanLs)

	select {
	case err = <-errChanLs:
		t.Errorf("Got error: %s", err.Error())
		return
	case ls = <-doneChanLs:
	}

	deleteResponse, err := ftpConn.DeleteFile("tmp_renamed.txt")
	if err != nil {
		t.Errorf("Got delete error: %s", err.Error())
		return
	}

	t.Logf("RENAME resp: %s", responseRename.String())
	t.Logf("LIST resp: %s", ls[0])
	t.Logf("DELETE resp: %s", deleteResponse.String())

}

func TestStoreAbort(t *testing.T) {

	ftpConn, _, err := authenticatedConn()

	if err != nil {
		t.Fatalf("Conn error: %s", err.Error())
	}

	defer ftpConn.Quit()

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

	doneChanStore := make(chan struct{}, 1)
	abortChanStore := make(chan struct{}, 1)
	errChanStore := make(chan error, 1)

	// writing immediately to
	// abortChanStore to be sure
	// that the function will abort immediately,
	// if not, because of the short file
	// dimension, one sending will be enough and
	// the function will never really use the abort.
	abortChanStore <- struct{}{}

	go ftpConn.Store("tmp.txt", FTP_MODE_ACTIVE, doneChanStore, abortChanStore, errChanStore)

	select {
	case err = <-errChanStore:
		t.Errorf("Got store error: %s", err.Error())
		return
	case <-doneChanStore:
	}
}

func TestRetrAbort(t *testing.T) {
	ftpConn, _, err := authenticatedConn()

	if err != nil {
		t.Fatalf("Conn error: %s", err.Error())
	}

	defer ftpConn.Quit()

	fileContent := []byte("hello this is an example")
	file, err := os.Create("tmp.txt")
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	defer os.Remove("tmp.txt")

	_, err = file.Write(fileContent)
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	doneChanStore := make(chan struct{}, 1)
	abortChanStore := make(chan struct{}, 1)
	errChanStore := make(chan error, 1)

	go ftpConn.Store("tmp.txt", FTP_MODE_ACTIVE, doneChanStore, abortChanStore, errChanStore)

	select {
	case err = <-errChanStore:
		t.Errorf("Got store error: %s", err.Error())
		return
	case <-doneChanStore:
	}

	doneChanRetr := make(chan struct{}, 1)
	abortChanRetr := make(chan struct{}, 1)
	errChanRetr := make(chan error, 1)

	abortChanRetr <- struct{}{}
	go ftpConn.Retrieve(FTP_MODE_ACTIVE, "tmp.txt", "temp.txt", doneChanRetr, abortChanRetr, errChanRetr)

	select {
	case err = <-errChanRetr:
		t.Errorf("Got retr error: %s", err.Error())
		return
	case <-doneChanRetr:
		// checking that the file has been deleted.

		_, err := os.Open("temp.txt")
		if !os.IsNotExist(err) {
			t.Errorf("The file exists?: %s", err.Error())
			return
		}
	}
}
