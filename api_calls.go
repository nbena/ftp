/* ftp
   Copyright (C) 2018 Nicola Bena

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package ftp

import (
	"bufio"
	"crypto/tls"
	"errors"
	"strconv"
	"time"
)

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

	response, err := f.writeCommandAndGetResponse("USER " + f.config.Username + "\r\n")
	if err != nil {
		return nil, err
	}
	if response.Code != UsernameOk {
		return nil, newUnexpectedCodeError(UsernameOk, response.Code)
	}

	response, err = f.writeCommandAndGetResponse("PASS " + f.config.Password + "\r\n")
	if err != nil {
		return nil, err
	}
	return unexpectedErrorOrResponse(LoginOk, response)
}

//Quit close the current FTP session, it means that every transfer in progress
//is closed too.
func (f *Conn) Quit() (*Response, error) {
	response, err := f.writeCommandAndGetResponse("QUIT\r\n")
	if err != nil {
		return nil, err
	}
	if response.Code != QuitOk {
		return nil, newUnexpectedCodeError(QuitOk, response.Code)
	}

	err = f.control.Close()
	return response, err
}

// DeleteFile deletes the file.
func (f *Conn) DeleteFile(filepath string) (*Response, error) {
	resp, err := f.writeCommandAndGetResponse("DELE " + filepath + "\r\n")
	if err != nil {
		return nil, err
	}
	return unexpectedErrorOrResponse(DeleteFileOk, resp)
}

// MkDir creates a directory.
func (f *Conn) MkDir(name string) (*Response, error) {
	resp, err := f.writeCommandAndGetResponse("MKD " + name + "\r\n")
	if err != nil {
		return nil, err
	}
	return unexpectedErrorOrResponse(MkDirOk, resp)
}

// DeleteDir deletes a directory.
func (f *Conn) DeleteDir(name string) (*Response, error) {
	resp, err := f.writeCommandAndGetResponse("RMD " + name + "\r\n")
	if err != nil {
		return nil, err
	}
	return unexpectedErrorOrResponse(DeleteDirOk, resp)
}

// Cd change the working directory.
func (f *Conn) Cd(path string) (*Response, error) {
	resp, err := f.writeCommandAndGetResponse("CWD " + path + "\r\n")
	if err != nil {
		return nil, err
	}
	return unexpectedErrorOrResponse(CdOk, resp)
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
	if response.Code != SizeOk {
		return nil, 0, newUnexpectedCodeError(SizeOk, response.Code)
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
	if response.Code != LastModificationTimeOk {
		return nil, nil, newUnexpectedCodeError(LastModificationTimeOk, response.Code)
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
	if response.Code != PwdOk {
		return nil, "", newUnexpectedCodeError(PwdOk, response.Code)
	}
	directory, err := getPwd(response)
	return response, directory, err
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

// func (f *Conn) auth(tls bool) (*Response, error) {
// 	var response *Response
// 	var err error
// 	if tls {
// 		response, err = f.AuthTLS()
// 	} else {
// 		response, err = f.AuthSSL()
// 	}
// 	return response, err
// }

// AuthTLS issues an AuthTLS command.
// If the control connection is already TLS-ed an error will be
// thrown, containing ftp.AlreadyTLS. If failback,
// AuthSSL will be tried.
func (f *Conn) AuthTLS(failback bool) (*Response, error) {
	response, err := f.writeCommandAndGetResponse("AUTH TLS\r\n")
	if err != nil {
		return nil, err
	}

	// log.Printf("Response: %s", response.String())

	if failback && response.Code != 234 { // tryssl
		return f.AuthSSL()
	}
	if response.Code != 234 {
		return nil, errors.New(response.Error())
	}

	// everything is fine...
	f.control = tls.Client(f.control, f.config.tlsConfig)
	tlsConn := f.control.(*tls.Conn)
	err = tlsConn.Handshake()

	// creating the new reader.
	f.controlRw = bufio.NewReadWriter(
		bufio.NewReader(f.control),
		bufio.NewWriter(f.control),
	)

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
	doneChan := make(chan struct{}, 1)
	abortChan := make(chan struct{}, 1)
	startingChan := make(chan struct{}, 1)
	errChan := make(chan error, 1)

	f.internalStore(mode, src, dst, doneChan, abortChan, startingChan,
		errChan, nil, false)

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

	f.internalRetr(mode, src, dst, doneChan, abortChan, startingChan, errChan, nil)

	var returned error

	select {
	case <-doneChan:
	case err := <-errChan:
		returned = err
	}
	return returned
}

// Store loads a file to the server. The file is 'filepath',
// specifies which FTP you want to use,
// doneChan notifies when transfering is finished, if an error
// occurs, it will be written on errChan, casuing the function to immediately
// exit.
// If you want to abort this transfering, write to abort. That channel
// should be buffered, because this function don't check until it starts transfering.
// When the abort command has been sent we wait for a response,
// then the channel is closed and an empty struct wil be written on
// doneChan.
// Sending something to abortChan, will not cause the file to be deleted,
// because that channel is checked AFTER THE ISSUING OF THE COMMAND
// (but before the transfer).
// In order to delete the file, pass the option.
func (f *Conn) Store(
	mode Mode,
	src string,
	dst string,
	doneChan chan<- struct{},
	abortChan <-chan struct{},
	startingChan chan<- struct{},
	errChan chan<- error,
	onEachChan chan<- struct{},
	deleteIfAbort bool,
) {

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
		deleteIfAbort)
}

// Retrieve download a file located at filepathSrc to filepathDest.
// When finished, it writes into doneChan. Any error, that'll make it immediately exits,
// is written into errChan.
func (f *Conn) Retrieve(mode Mode,
	filepathSrc,
	filepathDest string,
	doneChan chan<- struct{},
	abortChan <-chan struct{},
	startingChan chan<- struct{},
	errChan chan<- error,
	onEachChan chan<- struct{},
) {

	f.internalRetr(mode,
		filepathSrc,
		filepathDest,
		doneChan,
		abortChan,
		startingChan,
		errChan,
		onEachChan,
	)
}
