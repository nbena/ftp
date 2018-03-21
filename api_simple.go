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
