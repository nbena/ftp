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
	"errors"
	"io"
	"net"
	"os"
	"path"
	"strconv"
	"sync"
	"time"
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
	control net.Conn
	// controlReader *bufio.Reader
	controlRw *bufio.ReadWriter
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

	if _, err := f.writeCommandAndGetResponse("USER " + f.config.Username + "\r\n"); err != nil {
		return nil, err
	}

	response, err := f.writeCommandAndGetResponse("PASS " + f.config.Password + "\r\n")
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

// Store loads a file to the server. The file is 'filepath',
// specifies which FTP you want to use,
// doneCha notifies when transfering is finished, if an error
// occurs, it will be written on errChan, casuing the function to immediately
// exit.
// If you want to abort this transfering, write to abort. That channel
// should be buffered, because this function don't check until it starts transfering.
// When the abort command has been sent we wait for a response,
// then the channel is closed and an empty struct wil be written on
// doneChan.
func (f *Conn) Store(filepath string, mode Mode,
	doneChan chan<- struct{},
	abortChan <-chan struct{},
	errChan chan error) {

	/*
				This command tells the server to abort the previous FTP
		service command and any associated transfer of data.  The
		abort command may require "special action", as discussed in
		the Section on FTP Commands, to force recognition by the
		server.  No action is to be taken if the previous command
		has been completed (including data transfer).  The control
		connection is not to be closed by the server, but the data
		connection must be closed.

		There are two cases for the server upon receipt of this
		command: (1) the FTP service command was already completed,
		or (2) the FTP service command is still in progress.

			 In the first case, the server closes the data connection
			 (if it is open) and responds with a 226 reply, indicating
			 that the abort command was successfully processed.

			 In the second case, the server aborts the FTP service in
			 progress and closes the data connection, returning a 426
			 reply to indicate that the service request terminated
			 abnormally.  The server then sends a 226 reply,
			 indicating that the abort command was successfully
			 processed.
	*/

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

	for n < int(info.Size()) {

		select {
		// starting polling on abort
		case <-abortChan:
			// it's not completely correct to close here the data channel,
			// but some server will expect the client to do this.
			sender.Close()
			response, err := f.writeCommandAndGetResponse("ABOR\r\n")
			if err != nil {
				errChan <- err
				return
			}

			// if 426 we have to wait for another response.
			// if 226 the transfer is complete
			if response.Code == 426 && response.Code != 226 {
				// ok, wait for another.
				_, err := f.getFtpResponse()
				if err != nil {
					// very bad
					errChan <- err
					return
				}
			}
			doneChan <- struct{}{}
			return

		default: // no aborting
			read, err := file.Read(buffer)
			if err != nil {
				sender.Close()
				errChan <- err
				return
			}
			n += read
			_, err = sender.Write(buffer[:read])
			if err != nil {
				sender.Close()
				errChan <- err
				return
			}
		}
	}

	// until I close the data connection it doesn't answer me.
	sender.Close()

	// when completed reading response.
	if _, err := f.getFtpResponse(); err != nil {
		errChan <- err
		return
	}

	doneChan <- struct{}{}
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

// Size returns the size of the specified file. The size
// is not the size of the file but the number of bytes that
// will be transmitted if the file would have been downloaded.
// According to RFC 3659, a 213 code must be returned if the
// request is ok. If another code is returned, an error will be thrown.
func (f *Conn) Size(file string) (*Response, int, error) {
	response, err := f.writeCommandAndGetResponse("SIZE " + file + "\r\n")
	if err != nil {
		return nil, 0, err
	}
	if response.Code != 213 {
		return nil, 0, errors.New(response.Error())
	}
	// the size is the message.
	size, err := strconv.Atoi(response.Msg)
	if err != nil {
		return nil, 0, err
	}
	return response, size, nil
}

// LastModificationTime returns the last modification file of the given file in
// UTC format. The raw response is accessible, as well as the date (for sure)
// and an eventual error occured.
func (f *Conn) LastModificationTime(file string) (*Response, *time.Time, error) {
	response, err := f.writeCommandAndGetResponse("MDTM " + file + "\r\n")
	if err != nil {
		return nil, nil, err
	}
	date, err := response.getTime()
	return response, date, err
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
func (f *Conn) Retrieve(mode Mode, filepathSrc, filepathDest string,
	doneChan chan<- struct{},
	abortChan <-chan struct{},
	errChan chan<- error) {

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
	loop := true

	for loop {

		select {

		case <-abortChan:
			receiver.Close()
			var response *Response //declaring here just to prevent go vet.
			response, err = f.writeCommandAndGetResponse("ABOR\r\n")
			if err != nil {
				errChan <- err

				os.Remove(file.Name()) //skipping the error.
				return
			}

			// if 426 we have to wait for another response.
			// if 226 the transfer is complete
			if response.Code == 426 && response.Code != 226 {
				// ok, wait for another.
				_, err = f.getFtpResponse()
				if err != nil {
					// very bad
					errChan <- err
					return
				}
			}

			err = os.Remove(file.Name())
			if err != nil {
				errChan <- err
			} else {
				doneChan <- struct{}{}
			}
			return

		default:

			_, err = receiver.Read(buffer)
			if err != nil && err == io.EOF {
				// I know that it is ugly
				// but in this way we can skip some if.
				// EOF means the connection has been closed.
				loop = false
				continue // before there was a break but it'll make
				// exiting just from the select.
			} else if err != nil {
				receiver.Close()
				errChan <- err
				return
			}

			// so we can skip null bytes that are added
			/// to fill the buffer size.
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
	}

	// now getting the response.
	// it's not unreachable code...
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
	if _, err := f.writeCommandAndGetResponse("RNFR " + from + "\r\n"); err != nil {
		// fmt.Printf("First: %s\n", response.String())
		return nil, err
	}
	return f.writeCommandAndGetResponse("RNTO " + to + "\r\n")
}
