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
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"strconv"
	"sync"
)

// Mode is used as a constant
//to specify which type of
//default ftp mode should be used
//(active or passive).
type Mode int

const (
	// FTP_MODE_ACTIVE means that for default the
	// active modality will be used.
	FTP_MODE_ACTIVE = Mode(1)
	// FTP_MODE_PASSIVE means that the passive mode
	// will be used.
	FTP_MODE_PASSIVE = Mode(2)

	// This needs to be implemented.
	FTP_MODE_IND = Mode(0)
)

// Config contains the
//parameter used for the connection.
type Config struct {
	//The default modality (active or passive).
	DefaultMode Mode

	//Where to put response from the server.
	//Usually it is set to /dev/null or os.Stdin.
	// ResponseFile *os.File
	//Optionally, the tls configuration to use for
	//the connection.
	TLSConfig *tls.Config
	LocalIP   net.IP
	LocalPort int
	Username  string
	Password  string
	FirstPort int
}

// Conn represents the top level object.
type Conn struct {
	control       net.Conn
	controlReader *bufio.Reader
	// data            *lane.Queue // net.Conn
	// listeners       *lane.Queue //net.Listener
	// listenersParams *lane.Queue
	//dataRW       *bufio.ReadWriter
	//controlRW    []*bufio.ReadWriter
	//lastResponse string
	config       *Config
	lastUsedPort int
	portLock     sync.Mutex
	lastUsedMod  Mode
	// rand   *rand.Rand
}

// Response is the response from the server.
type Response struct {
	//Response code received from the server.
	Code int
	//Msg sent from the server.
	Msg string
}

func (r *Response) Error() string {
	return r.String()
}

func (r *Response) String() string {
	return strconv.Itoa(r.Code) + ": " + r.Msg
}

// IsFtpError returns true if the response represents
// an error. That means that the code is >=500 && < 600.
func (r *Response) IsFtpError() bool {
	return r.Code >= 500 && r.Code < 600
}

// IsFileNotExists check if the code is 450.
func (r *Response) IsFileNotExists() bool {
	return r.Code == 450
}

// Dial connects to the ftp server.
func Dial(remote string, config *Config) (*Conn, *Response, error) {
	return internalDial(remote, config)
}

// DialAndAuthenticate connects to the server and
// authenticates with it.
func DialAndAuthenticate(remote string, config *Config) (*Conn, *Response, error) {
	conn, _, err := internalDial(remote, config)
	if err != nil {
		// log.Printf("catch this as well")
		return nil, nil, err
	}
	resp, err := conn.Authenticate()
	if err != nil {
		return nil, nil, err
	}
	return conn, resp, nil
}

// Authenticate sends credentials to the serve. It will
// return an error if an error occurs in sending/receiving or
// if the credential are wrong.
func (f *Conn) Authenticate() (*Response, error) {
	f.writeCommand("USER " + f.config.Username + "\r\n")
	_, err := f.getFtpResponse()

	if err != nil {
		return nil, err
	}

	f.writeCommand("PASS " + f.config.Password + "\r\n")
	response, err := f.getFtpResponse()

	if err != nil {
		return nil, err
	}

	return response, nil
}

//Quit close the current FTP session, it means that every transfer in progress
//is closed too.
func (f *Conn) Quit() (*Response, error) {
	response, err := f.writeCommandAndGetResponse("QUIT\r\n")
	if err != nil {
		return nil, err
	}

	err = f.control.Close()
	return response, err
}

// Store loads a file to the server.
func (f *Conn) Store(filepath string, mode Mode, doneChan chan<- struct{}, errChan chan error) {

	var sender io.WriteCloser

	_, fileName := path.Split(filepath)

	if mode == FTP_MODE_ACTIVE {

		listener, err := f.openListener()
		if err != nil {
			errChan <- err
			return
		}
		defer listener.Close()

		if _, err = f.writeCommandAndGetResponse("STOR " + fileName + "\r\n"); err != nil {
			errChan <- err
			return
		}

		sender, err = listener.Accept()
		if err != nil {
			errChan <- err
			return
		}

	} else if mode == FTP_MODE_PASSIVE {

		addr, err := f.pasvGetAddr()
		if err != nil {
			errChan <- err
			return
		}

		// write command
		if _, err = f.writeCommandAndGetResponse("STOR " + fileName + "\r\n"); err != nil {
			errChan <- err
			return
		}

		sender, err = f.connectToAddr(addr)
		if err != nil {
			errChan <- err
			return
		}

	}

	var n int
	file, err := os.Open(filepath)
	if err != nil {
		errChan <- err
		return
	}

	info, err := file.Stat()
	if err != nil {
		errChan <- err
		return
	}

	buffer := make([]byte, 1024)
	// fmt.Printf("buffer created")

	for n < int(info.Size()) {

		// reading
		// fmt.Printf("reading")
		read, err := file.Read(buffer)
		if err != nil {
			sender.Close()
			errChan <- err
			return
		}
		n += read
		// fmt.Printf("read")

		// sending data
		// written, err := sender.Write(buffer)
		_, err = sender.Write(buffer[:read])
		// fmt.Printf("written")
		if err != nil {
			// fmt.Printf("write error")
			sender.Close()
			errChan <- err
			return
		}

		// if written != len(buffer) {
		// 	done <- fmt.Sprintf("Partial write: want %d, got %d", len(buffer), written)
		// 	return
		// }
	}

	// until I close the data connection it doesn't answer me.
	sender.Close()

	// when completed reading response.
	if _, err := f.getFtpResponse(); err != nil {
		errChan <- err
		return
	}

	doneChan <- struct{}{}
	// _, err = f.getFtpResponse()
	// if err != nil {
	// 	done <- err.Error()
	// } else {
	// 	done <- ""
	// }
}

// DeleteFile deletes the file.
func (f *Conn) DeleteFile(filepath string) (*Response, error) {
	return f.writeCommandAndGetResponse("DELE " + filepath + "\r\n")
}

// MkDir creates a directory.
func (f *Conn) MkDir(name string) (*Response, error) {
	return f.writeCommandAndGetResponse("MKD " + name + "\r\n")
}

// DeleteDir deletes a directory.
func (f *Conn) DeleteDir(name string) (*Response, error) {
	return f.writeCommandAndGetResponse("RMD " + name + "\r\n")
}

// Cd change the working directory.
func (f *Conn) Cd(path string) (*Response, error) {
	return f.writeCommandAndGetResponse("CWD " + path + "\r\n")
}

// Ls performs a LIST on the current directory.
// The result is written on doneChan, one row per item. Eventual errors will be
// written on errChan, causing the function to immediately exit.
func (f *Conn) Ls(mode Mode, doneChan chan<- []string, errChan chan<- error) {
	f.internalLs(mode, "", doneChan, errChan)
}

// LsDir performs a LIST on the given directory/file.
// The result is written on doneChan, one row per item. Eventual errors will be
// written on errChan, causing the function to immediately exit.
func (f *Conn) LsDir(mode Mode, path string, doneChan chan<- []string, errChan chan<- error) {
	f.internalLs(mode, path, doneChan, errChan)
}

// Pwd returns the current working directory.
// It returns the raw response too.
func (f *Conn) Pwd() (*Response, string, error) {
	response, err := f.writeCommandAndGetResponse("PWD\r\n")
	if err != nil {
		return nil, "", err
	}
	directory, err := getPwd(response)
	return response, directory, err
}

// Retrieve download a file located at filepathSrc to filepathDest.
// When finished, it writes into doneChan. Any error, that'll make it immediately exits,
// is written into errChan.
func (f *Conn) Retrieve(mode Mode, filepathSrc, filepathDest string, doneChan chan<- struct{}, errChan chan<- error) {

	var receiver io.ReadCloser

	if mode == FTP_MODE_ACTIVE {

		//opening a listener.
		listener, err := f.openListener()
		if err != nil {
			errChan <- err
			return
		}
		defer listener.Close()

		// sending command.
		if _, err = f.writeCommandAndGetResponse("RETR " + filepathSrc + "\r\n"); err != nil {
			errChan <- err
			return
		}

		// accept connection.
		receiver, err = listener.Accept()
		if err != nil {
			errChan <- err
			return
		}
	} else if mode == FTP_MODE_PASSIVE {

		addr, err := f.pasvGetAddr()
		if err != nil {
			errChan <- err
			return
		}

		// write command
		if _, err = f.writeCommandAndGetResponse("RETR " + filepathSrc + "\r\n"); err != nil {
			errChan <- err
			return
		}

		receiver, err = f.connectToAddr(addr)
		if err != nil {
			errChan <- err
			return
		}
	}

	file, err := os.Create(filepathDest)
	if err != nil {
		errChan <- err
		return
	}

	// starting reading into receiver
	buffer := make([]byte, 1024)

	for {
		_, err = receiver.Read(buffer)
		if err != nil && err == io.EOF {
			// ending.
			break
		} else if err != nil {
			errChan <- err
			return
		}

		// so we can skip null bytes.
		index := bytes.IndexByte(buffer, 0)
		if index == -1 {
			index = len(buffer)
		}

		if _, err = file.Write(buffer[:index]); err != nil {
			// closing the connection as well
			receiver.Close()
			errChan <- err
			return
		}
	}

	// now getting the response.
	_, err = f.getFtpResponse()
	if err != nil {
		errChan <- err
		return
	}

	doneChan <- struct{}{}
}

// Rename renames a file called 'from' to a file called 'to'.
// It returns just the last response.
func (f *Conn) Rename(from, to string) (*Response, error) {
	if response, err := f.writeCommandAndGetResponse("RNFR " + from + "\r\n"); err != nil {
		fmt.Printf("First: %s\n", response.String())
		return nil, err
	}
	return f.writeCommandAndGetResponse("RNTO " + to + "\r\n")
}
