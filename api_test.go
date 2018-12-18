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
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func authenticatedConn() (*Conn, *Response, error) {
	return DialAndAuthenticate("localhost:2121", &Config{
		DefaultMode: ActiveMode,
		Username:    "anonymous",
		Password:    "c@b.com",
		LocalIP:     net.IP([]byte{127, 0, 0, 1}),
	})
}

func TestDial(t *testing.T) {

	ftpConn, resp, err := Dial("localhost:2121", &Config{
		DefaultMode: ActiveMode,
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

func internalFilesOps(t *testing.T, mode Mode, useSimple bool, bufferSize int) {
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
		t.Error(err.Error())
		return
	}

	doneChanStore := make(chan struct{}, 1)
	abortChanStore := make(chan struct{}, 1)
	startingChanStore := make(chan struct{}, 1)
	errChanStore := make(chan error, 1)
	onEachChanStore := make(chan int, 10)

	stat, err := file.Stat()
	if err != nil {
		t.Error(err.Error())
		return
	}

	size := stat.Size()

	if useSimple == false {
		go ftpConn.Store(mode, "tmp.txt", "tmp.txt", doneChanStore, abortChanStore,
			startingChanStore, errChanStore, onEachChanStore, false, bufferSize)

		loop := true
		for loop {
			select {
			case err = <-errChanStore:
				t.Errorf("Got error: %s", err.Error())
				return
			case <-doneChanStore:
				t.Logf("Store transfer has finished\n")
				loop = false
			case <-startingChanStore:
				t.Logf("Store transfer has started\n")
			}
		}
	} else {

		// use simple api
		if err = ftpConn.StoreSimple(mode, "tmp.txt", "tmp.txt"); err != nil {
			t.Errorf("Got error: %s", err.Error())
			return
		}

	}

	doneChanRetr := make(chan struct{}, 1)
	abortChanRetr := make(chan struct{}, 1)
	startingChanRetr := make(chan struct{}, 1)
	errChanRetr := make(chan error, 1)
	onEachChanRetr := make(chan int, 10)

	if useSimple == false {

		go ftpConn.Retrieve(mode, "tmp.txt", "temp_get.txt",
			doneChanRetr,
			abortChanRetr,
			startingChanRetr,
			errChanRetr,
			onEachChanRetr,
			bufferSize,
		)

		loop := true

		for loop {
			select {
			case err = <-errChanRetr:
				t.Errorf("Got error: %s", err.Error())
				return
			case <-startingChanRetr:
				t.Logf("Retr transfer has started\n")
			case <-doneChanRetr:
				t.Logf("Retr transfer has started\n")
				loop = false
			}
		}
	} else {

		// use simple api
		if err = ftpConn.RetrSimple(mode, "tmp.txt", "temp_get.txt"); err != nil {
			t.Errorf("Got error: %s", err)
			return
		}

	}

	//reading downloaded file.
	var content []byte // just to prevent go vet
	content, err = ioutil.ReadFile("temp_get.txt")
	defer os.Remove("temp_get.txt")
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
		return
	}
	if !reflect.DeepEqual(fileContent, content) {
		t.Errorf("Mismatched files")
		t.Errorf("string content:\n'%s'\n'%s'", string(fileContent), string(content))
		t.Logf("byte content:\n%v\n%v", fileContent, content)
	}

	// now checking the size of the uploaded file.
	sizeResponse, gotSize, err := ftpConn.Size("tmp.txt")
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
		return
	}

	if int(size) != gotSize {
		t.Errorf("Mismatched size, want: %d, got: %d", size, gotSize)
		return
	}

	response, err := ftpConn.DeleteFile("tmp.txt")
	if err != nil {
		t.Errorf("Got error: %s", err.Error())
		return
	}

	t.Logf("SIZE resp: %s", sizeResponse.String())
	t.Logf("DELE resp: %s", response.String())
}

func TestFileOpsActiveSync(t *testing.T) {
	internalFilesOps(t, ActiveMode, true, 0)
}

func TestFileOpsPassiveSync(t *testing.T) {
	internalFilesOps(t, PassiveMode, true, 0)
}

func TestFileOpsActiveAsyncDefaultBuffer(t *testing.T) {
	internalFilesOps(t, ActiveMode, false, 0)
}

func TestFileOpsPassiveAsyncDefaultBuffer(t *testing.T) {
	internalFilesOps(t, PassiveMode, false, 0)
}

func TestFileOpsActiveAsyncCustomBuffer(t *testing.T) {
	internalFilesOps(t, ActiveMode, false, MaxAllowedBufferSize)
}

func TestFileOpsPassiveAsyncCustomBuffer(t *testing.T) {
	internalFilesOps(t, PassiveMode, false, MaxAllowedBufferSize)
}

func internalDirOps(t *testing.T, mode Mode, useSimple bool) {
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

	// var responseStr string

	if useSimple == false {
		go ftpConn.Ls(mode, doneChanLs, errChanLs)
		select {
		case lsResponse = <-doneChanLs:
		case err = <-errChanLs:
			t.Errorf("Got error: %s", err.Error())
			return
		}

		go ftpConn.LsDir(mode, "temp", doneChanDir, errChanDir)
		select {
		case lsDirResponse = <-doneChanDir:
		case err = <-errChanDir:
			t.Errorf("Got error: %s", err.Error())
			return
		}

		// this will throw an error

		doneChanLsErr := make(chan []string)
		errChanLsErr := make(chan error, 1)

		go ftpConn.LsDir(mode, "tmp_renamed", doneChanLsErr, errChanLsErr)

		select {
		case err = <-errChanLsErr:
			//if !inverseResponse(err.Error()).IsFileNotExists() {
			// building a response from this error.
			response, err := newFtpResponse(err.Error())
			if err != nil {
				t.Errorf(err.Error())
				return
			}
			if response.Code != FileUnavailable {
				t.Errorf("Got error: %s", err.Error())
				return
			}
		case <-doneChanLsErr:
			t.Errorf("Got 0 error while expecting one")
			return
		}
	} else {
		lsResponse, err = ftpConn.LsSimple(mode)
		if err != nil {
			t.Errorf("Got error: %s", err.Error())
			return
		}
		lsDirResponse, err = ftpConn.LsDirSimple(mode, "temp")
		if err != nil {
			t.Errorf("Got error: %s", err.Error())
			return
		}
		_, err = ftpConn.LsDirSimple(mode, "tmp_renamed")
		if err != nil {
			response, err := newFtpResponse(err.Error())
			if err != nil {
				t.Errorf(err.Error())
				return
			}
			if response.Code != FileUnavailable {
				t.Errorf("Got error: %s", err.Error())
				return
			}
		} else {
			t.Errorf("Got 0 error while expecting one")
			return
		}
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

func TestDirOpsActiveSimple(t *testing.T) {
	internalDirOps(t, ActiveMode, true)
}

func TestDirOpsActiveNoSimple(t *testing.T) {
	internalDirOps(t, ActiveMode, false)
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
	startingChanStore := make(chan struct{}, 1)
	errChanStore := make(chan error, 1)
	onEachChanStore := make(chan int, 10)

	go ftpConn.Store(ActiveMode, "tmp.txt", "tmp.txt", doneChanStore, abortChanStore,
		startingChanStore, errChanStore, onEachChanStore, false, 0)

	loop := true
	for loop {
		select {
		case err = <-errChanStore:
			t.Errorf("Got store error: %s", err.Error())
			return
		case <-doneChanStore:
			t.Logf("Store transfer has finished\n")
			loop = false
		case <-startingChanStore:
			t.Logf("Store transfer has started\n")
		}
	}

	responseRename, err := ftpConn.Rename("tmp.txt", "tmp_renamed.txt")
	if err != nil {
		t.Errorf("Got rename error: %s", err.Error())
		return
	}

	doneChanLs := make(chan []string)
	errChanLs := make(chan error, 1)
	var ls []string

	go ftpConn.LsDir(ActiveMode, "tmp_renamed.txt", doneChanLs, errChanLs)

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

func internalStoreAbort(t *testing.T, beforeOrAfter bool) {
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
	startingChanStore := make(chan struct{}, 1)
	errChanStore := make(chan error, 2)
	onEachChanStore := make(chan int, 10)

	// writing immediately to
	// abortChanStore to be sure
	// that the function will abort immediately,
	// if not, because of the short file
	// dimension, one sending will be enough and
	// the function will never really use the abort.
	if beforeOrAfter {
		abortChanStore <- struct{}{}
	}

	go ftpConn.Store(ActiveMode, "tmp.txt", "tmp.txt", doneChanStore, abortChanStore,
		startingChanStore, errChanStore, onEachChanStore, true, 0)

	select {
	case err = <-errChanStore:
		t.Errorf("Got store error: %s", err.Error())
		return
	case <-doneChanStore:
		if beforeOrAfter {
			// checking that the file has been deleted.
			doneChanLs := make(chan []string, 1)
			errChanLs := make(chan error, 1)
			ftpConn.Ls(ActiveMode, doneChanLs, errChanLs)
			select {
			case err = <-errChanLs:
				t.Errorf("Got LS error: %s", err.Error())
				return
			case ls := <-doneChanLs:
				for _, entry := range ls {
					if strings.Contains(entry, "tmp.txt") {
						t.Errorf("Found file: %s", entry)
						return
					}
				}
			}
		} else {
			abortChanStore <- struct{}{}
		}
	}
}

func TestStoreAbortBefore(t *testing.T) {
	internalStoreAbort(t, true)
}

func TestStoreAbortAfter(t *testing.T) {
	internalStoreAbort(t, false)
}

func internalRetrAbort(t *testing.T, beforeOrAfter bool) {
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
	startingChanStore := make(chan struct{}, 1)
	errChanStore := make(chan error, 1)
	onEachChanStore := make(chan int, 10)

	go ftpConn.Store(ActiveMode, "tmp.txt", "tmp.txt", doneChanStore, abortChanStore,
		startingChanStore, errChanStore, onEachChanStore, false, 0)

	select {
	case err = <-errChanStore:
		t.Errorf("Got store error: %s", err.Error())
		return
	case <-doneChanStore:
	}

	doneChanRetr := make(chan struct{}, 1)
	abortChanRetr := make(chan struct{}, 1)
	startingChanRetr := make(chan struct{}, 1)
	errChanRetr := make(chan error, 1)
	onEachChanRetr := make(chan int, 10)

	if beforeOrAfter {
		abortChanRetr <- struct{}{}
	}

	go ftpConn.Retrieve(ActiveMode, "tmp.txt", "temp.txt", doneChanRetr,
		abortChanRetr, startingChanRetr, errChanRetr, onEachChanRetr, 0)

	select {
	case err = <-errChanRetr:
		t.Errorf("Got retr error: %s", err.Error())
		return
	case <-doneChanRetr:
		// checking that the file has been deleted.

		if beforeOrAfter {
			_, err := os.Open("temp.txt")
			if !os.IsNotExist(err) {
				t.Errorf("The file exists?: %s", err.Error())
				return
			}
		} else {
			abortChanRetr <- struct{}{}
			os.Remove("temp.txt")
		}
	}
}

func TestRetrAbortBefore(t *testing.T) {
	internalRetrAbort(t, true)
}

func TestRetrAbortAfter(t *testing.T) {
	internalRetrAbort(t, false)
}

func TestParseTimeNoop(t *testing.T) {

	response, err := newFtpResponse("213 20180226133244.000")
	if err != nil {
		t.Errorf(err.Error())
	}

	date, err := response.getTime()
	if date.Year() != 2018 || date.Month() != time.February || date.Day() != 26 || date.Hour() != 13 || date.Minute() != 32 || date.Second() != 44 || date.Nanosecond() != 00 {
		t.Errorf("Error in date: %s", date.String())
	}

	ftpConn, _, err := authenticatedConn()
	if err != nil {
		t.Fatalf(err.Error())
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
	startingChanStore := make(chan struct{}, 1)
	errChanStore := make(chan error, 1)
	onEachChanRetr := make(chan int, 10)

	go ftpConn.Store(ActiveMode, "tmp.txt", "tmp.txt", doneChanStore, abortChanStore,
		startingChanStore, errChanStore, onEachChanRetr, false, 0)

	loop := true
	for loop {
		select {
		case err = <-errChanStore:
			t.Errorf("Got store error: %s", err.Error())
			return
		case <-doneChanStore:
			t.Logf("Store transfer has finished\n")
			loop = false
		case <-startingChanStore:
			t.Logf("Store transfer has started\n")
		}
	}

	gotResponse, gotDate, err := ftpConn.LastModificationTime("tmp.txt")
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	//finally a noop
	noopResponse, err := ftpConn.Noop()
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	t.Logf("Raw: %s", gotResponse)
	t.Logf("Time: %s", gotDate.String())
	t.Logf("Noop: %s", noopResponse.String())
}

func TestAuthTLS(t *testing.T) {
	// we expect to fail
	// if tested against apache ftp server.

	ftpConn, _, err := DialAndAuthenticate("localhost:2121",
		&Config{
			Username: "anonymous",
			Password: "c@b.i",
			TLSOption: &TLSOption{
				AllowSSL:       true,
				ImplicitTLS:    false,
				AuthTLSOnFirst: true,
				AllowWeakHash:  true,
				SkipVerify:     true,
			},
			DefaultMode: ActiveMode,
		},
	)

	if err != nil {
		if strings.Contains(err.Error(), "handshake failure") ||
			strings.Contains(err.Error(), "bind: address already in use") {
			t.Logf("Got \"expected\" error from handshake: %s", err.Error())
		} else {
			t.Errorf(err.Error())
		}
	} else {
		ftpConn.Quit()
	}
	// even if handshake error is expected, it's still an error
	// so we can defer ONLY IF NO ERROR HAS HAPPENED.
	// if err == nil {
	//
	// 	// and defer is not really needed.
	// 	defer ftpConn1.Quit()
	// }
}

func TestBufferSize(t *testing.T) {

	ftpConn, _, err := authenticatedConn()
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	defer ftpConn.Quit()

	buffer := ftpConn.BufferSize()
	if buffer != bufferSize {
		t.Errorf("Expected buffer size of %d, got %d", bufferSize, buffer)
	}
}
