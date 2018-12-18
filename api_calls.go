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
	"bufio"
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"
)

// Dial connects to the ftp server using the given configuration,
// it returns a `Conn`, the server response, or an error.
// It only setups the TCP connection for the control channel, no credentials are sent.
func Dial(remote string, config *Config) (*Conn, *Response, error) {
	return internalDial(remote, config)
}

// DialAndAuthenticate connects to the server and
// authenticates with it.
func DialAndAuthenticate(remote string, config *Config) (*Conn, *Response, error) {
	conn, _, err := internalDial(remote, config)
	if err != nil {
		return nil, nil, err
	}
	resp, err := conn.Authenticate()
	if err != nil {
		return nil, nil, err
	}
	return conn, resp, nil
}

// Authenticate sends credentials to the server. It returns
// an error if an error occurs in sending/receiving or
// if the credential are wrong.
func (f *Conn) Authenticate() (*Response, error) {

	// Sending the username.
	response, err := f.writeCommandAndGetResponse("USER " + f.config.Username + "\r\n")
	if err != nil {
		return nil, err
	}
	// if it's not the response code we expect...
	if response.Code != UsernameOk {
		return nil, newUnexpectedCodeError(UsernameOk, response.Code)
	}

	// now sending the password.
	response, err = f.writeCommandAndGetResponse("PASS " + f.config.Password + "\r\n")
	if err != nil {
		return nil, err
	}
	return unexpectedErrorOrResponse(LoginOk, response)
}

// Quit close the current FTP session. Every transfer in progress will be
// aborted, then `QUIT` command is sent.
func (f *Conn) Quit() (*Response, error) {
	// data channels-related goroutines
	// listen on the internal context to see whether it's being cancelled.
	// So we send the cancel signal.
	f.cancel()
	// Now sending `QUIT`.
	response, err := f.writeCommandAndGetResponse("QUIT\r\n")
	if err != nil {
		return nil, err
	}
	if response.Code != QuitOk {
		return nil, newUnexpectedCodeError(QuitOk, response.Code)
	}
	// Closing the control channel.
	err = f.control.Close()
	return response, err
}

// DeleteFile deletes the file at the given path.
func (f *Conn) DeleteFile(filepath string) (*Response, error) {
	resp, err := f.writeCommandAndGetResponse("DELE " + filepath + "\r\n")
	if err != nil {
		return nil, err
	}
	return unexpectedErrorOrResponse(DeleteFileOk, resp)
}

// MkDir creates a directory named `name` ai the current path.
func (f *Conn) MkDir(name string) (*Response, error) {
	resp, err := f.writeCommandAndGetResponse("MKD " + name + "\r\n")
	if err != nil {
		return nil, err
	}
	return unexpectedErrorOrResponse(MkDirOk, resp)
}

// DeleteDir deletes the directory `name`.
func (f *Conn) DeleteDir(name string) (*Response, error) {
	resp, err := f.writeCommandAndGetResponse("RMD " + name + "\r\n")
	if err != nil {
		return nil, err
	}
	return unexpectedErrorOrResponse(DeleteDirOk, resp)
}

// Cd change the working directory to `path`.
func (f *Conn) Cd(path string) (*Response, error) {
	resp, err := f.writeCommandAndGetResponse("CWD " + path + "\r\n")
	if err != nil {
		return nil, err
	}
	return unexpectedErrorOrResponse(CdOk, resp)
}

// LsSimple performs a LIST on the current directory blocking the main goroutine.
func (f *Conn) LsSimple(mode Mode) ([]string, error) {
	// Here we'll get the result.
	doneChan := make(chan []string, 1)
	// Here we get errors, if any.
	errChan := make(chan error, 1)
	// we still use the internalLs.
	f.internalLs(mode, "", doneChan, errChan)

	var returnedError error
	var returnedDirs []string
	select {
	case returnedDirs = <-doneChan:
	case returnedError = <-errChan:
	}

	return returnedDirs, returnedError
}

// LsDirSimple performs a LIST on the given directory in a synchronous mode.
func (f *Conn) LsDirSimple(mode Mode, dir string) ([]string, error) {
	doneChan := make(chan []string, 1)
	errChan := make(chan error, 1)
	f.internalLs(mode, dir, doneChan, errChan)

	var returnedError error
	var returnedDirs []string
	select {
	case returnedDirs = <-doneChan:
	case returnedError = <-errChan:
	}

	return returnedDirs, returnedError
}

// Ls performs a LIST on the current directory.
// The result is written on doneChan, one row per element of the array.
//  Errors will be
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
// Returns the server response, the size, or an error.
func (f *Conn) Size(file string) (*Response, int, error) {
	response, err := f.writeCommandAndGetResponse("SIZE " + file + "\r\n")
	if err != nil {
		return nil, 0, err
	}
	if response.Code != SizeOk {
		return nil, 0, newUnexpectedCodeError(SizeOk, response.Code)
	}
	// Now parsing the response.
	size, err := strconv.Atoi(response.Msg)
	if err != nil {
		return nil, 0, err
	}
	return response, size, nil
}

// LastModificationTime returns the last modification time of the given file in
// UTC format. The raw response is accessible, as well as the parsed date.
func (f *Conn) LastModificationTime(file string) (*Response, *time.Time, error) {
	response, err := f.writeCommandAndGetResponse("MDTM " + file + "\r\n")
	if err != nil {
		return nil, nil, err
	}
	if response.Code != LastModificationTimeOk {
		return nil, nil, newUnexpectedCodeError(LastModificationTimeOk, response.Code)
	}
	date, err := response.getTime()
	return response, date, err
}

// Pwd returns the current working directory, As usual, the raw response is
// accessible as well.
func (f *Conn) Pwd() (*Response, string, error) {
	response, err := f.writeCommandAndGetResponse("PWD\r\n")
	if err != nil {
		return nil, "", err
	}
	if response.Code != PwdOk {
		return nil, "", newUnexpectedCodeError(PwdOk, response.Code)
	}
	directory, err := getPwd(response)
	return response, directory, err
}

// Rename renames a file called 'from' to a file called 'to'.
// This operation is not atomic (requires two messages) and atomicity
// is not handled by the library. Only the second response is returned.
func (f *Conn) Rename(from, to string) (*Response, error) {
	if _, err := f.writeCommandAndGetResponse("RNFR " + from + "\r\n"); err != nil {
		return nil, err
	}
	return f.writeCommandAndGetResponse("RNTO " + to + "\r\n")
}

// Noop issues a NOOP command.
func (f *Conn) Noop() (*Response, error) {
	resp, err := f.writeCommandAndGetResponse("NOOP\r\n")
	if err != nil {
		return nil, err
	}
	return unexpectedErrorOrResponse(NoopOk, resp)
}

// AuthSSL starts an SSL connection over the control channel.
// Support for SSL must be explicitely turn on into config
// with the option 'AllowSSL' AND in TLSConfig.
// If not, SSL will fail.  This is done for security reason,
// SSL is no longer secure, support for SSL3 must be explicitely set.
// Note that we expect a 234 code.
func (f *Conn) AuthSSL() (*Response, error) {
	if !f.config.TLSOption.AllowSSL {
		return nil, errors.New("Explicit support for SSL3 is required")
	}
	if f.config.tlsConfig.MinVersion > tls.VersionSSL30 {
		return nil, errors.New("Explicit support for SSL3 is required")
	}
	response, err := f.writeCommandAndGetResponse("AUTH SSL\r\n")
	if err != nil {
		return nil, err
	}
	if response.Code != 234 {
		return nil, errors.New(response.Error())
	}

	f.control = tls.Client(f.control, f.config.tlsConfig)

	tlsConn := f.control.(*tls.Conn)
	err = tlsConn.Handshake()

	// creating the new reader.
	f.controlRw = bufio.NewReadWriter(
		bufio.NewReader(f.control),
		bufio.NewWriter(f.control))

	return response, err
}

// AuthTLS issues an AuthTLS command.
// If the control connection is already TLS-ed an error will be
// thrown, containing ftp.AlreadyTLS. If failback,
// AuthSSL will be tried.
func (f *Conn) AuthTLS(failback, newConnOnFailure bool) (*Response, error) {
	response, err := f.writeCommandAndGetResponse("AUTH TLS\r\n")
	if err != nil {
		return nil, err
	}

	if failback && response.Code != 234 { // tryssl
		return f.AuthSSL()
	}
	if response.Code != 234 {
		return nil, errors.New(response.Error())
	}

	// if everything is fine...
	f.control = tls.Client(f.control, f.config.tlsConfig)
	tlsConn := f.control.(*tls.Conn)
	err = tlsConn.Handshake()

	if err != nil {
		// keeping the 'old' connection
		// so really nothing to do.
		f.Quit()
		newPort, _, _ := f.getRandomPort()
		host := strings.Split(f.control.LocalAddr().String(), ":")[0]

		remote := f.control.RemoteAddr().String()

		var ips []net.IP
		var newErr error
		var newConn *Conn

		ips, newErr = net.LookupIP(host)
		if newErr != nil {
			return nil, err
		}
		newConfig := f.config
		newConfig.LocalIP = ips[0]
		newConfig.LocalPort = newPort

		newConn, _, newErr = DialAndAuthenticate(remote, newConfig)
		if newErr != nil {
			return nil, newErr
		}
		newControl := newConn.control
		f.control = newControl
		f.controlRw = bufio.NewReadWriter(
			bufio.NewReader(f.control),
			bufio.NewWriter(f.control),
		)

	} else {
		// creating the new reader from the
		// TLS connection.
		f.controlRw = bufio.NewReadWriter(
			bufio.NewReader(f.control),
			bufio.NewWriter(f.control),
		)
	}

	return response, err
}

// StoreSimple is a simplified version of function Store which does not
// involves the use of channels, so it's suitable for uses when is not necessary
// to do an 'async' uploading.
func (f *Conn) StoreSimple(
	mode Mode,
	src,
	dst string,
) error {
	// Internally we still using the async version, so we
	// have to listen on the channel it uses.
	doneChan := make(chan struct{}, 1)
	// sends something here if you want to stop the storing.
	abortChan := make(chan struct{}, 1)
	startingChan := make(chan struct{}, 1)
	errChan := make(chan error, 1)

	f.internalStore(mode, src, dst, doneChan, abortChan, startingChan,
		errChan, nil, false, 0)

	var returned error

	select {
	case <-doneChan:
	case err := <-errChan:
		returned = err
	}
	return returned
}

// RetrSimple is a simplified version of function Retrieve which does not
// involves the use of channels, so it's suitable for uses when is not necessary
// to do an 'async' downloading.
func (f *Conn) RetrSimple(
	mode Mode,
	src,
	dst string,
) error {
	doneChan := make(chan struct{}, 1)
	abortChan := make(chan struct{}, 1)
	startingChan := make(chan struct{}, 1)
	errChan := make(chan error, 1)

	f.internalRetr(mode, src, dst, doneChan, abortChan, startingChan, errChan, nil, 0)

	var returned error

	select {
	case <-doneChan:
	case err := <-errChan:
		returned = err
	}
	return returned
}

// Store loads the file `src` to `dst`.
// Meaning of channels:
// - `doneChan`: you'll get back an empty struct when the loading is finished.
// - `abortChan`: sends something here to abort the transfer, should be buffered.
// - `startingChan`: you get back an empty struct when the transfer really starts.
// - `errChan` when an error happens. The transfer will be stopped as well.
// - `onEachChan`: the size of transferred bytes in each single transfer. Can be `nil`.
// If you want to delete the file if an abort happens, set `true` to `deleteIfAbort`.
// `bufferSize` is the optional custom buffer size to use for the transfer. Pass 0 to not care
// about it.
func (f *Conn) Store(
	mode Mode,
	src string,
	dst string,
	doneChan chan<- struct{},
	abortChan chan struct{},
	startingChan chan<- struct{},
	errChan chan<- error,
	onEachChan chan<- int,
	deleteIfAbort bool,
	bufferSize int,
) {

	// from the RFC:
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
	f.internalStore(mode,
		src,
		dst,
		doneChan,
		abortChan,
		startingChan,
		errChan,
		onEachChan,
		deleteIfAbort,
		bufferSize,
	)
}

// TODO see args order.
// Retrieve download a file located.
func (f *Conn) Retrieve(mode Mode,
	filepathSrc,
	filepathDest string,
	doneChan chan<- struct{},
	abortChan chan struct{},
	startingChan chan<- struct{},
	errChan chan<- error,
	onEachChan chan<- int,
	bufferSize int,
) {

	f.internalRetr(mode,
		filepathSrc,
		filepathDest,
		doneChan,
		abortChan,
		startingChan,
		errChan,
		onEachChan,
		bufferSize,
	)
}
