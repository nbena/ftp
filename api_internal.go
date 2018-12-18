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
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// func (r *Response) IsAborted() bool {
// 	return r.Code == 426
// }

// func (r *Response) IsSuccesfullyCompleted() bool {
// 	return r.Code == 226
// }
//
// func (r *Response) IsNotImplemented() bool {
// 	return r.Code == 502
// }
//
// // IsFailToAccomplish checks if response.Code == 550.
// // 550 is an error used when the server can parse our
// // request but can't serve it. For example when we request a
// // size for a set transfer mode which the file can't be sent over.
// // Another example is where modification time is not available.
// func (r *Response) IsFailToAccomplish() bool {
// 	return r.Code == 550
// }
//
// func (r *Response) IsNotSupported() bool {
// 	return r.Code == 431
// }
//

// IsFtpError returns true if the response represents
// an error. That means that the code is >=500 && < 600.
func (r *Response) IsFtpError() bool {
	return (r.Code >= 500 && r.Code < 600) || r.Code == FileUnavailable
}

//
// // IsFileNotExists check if the code is 450.
// func (r *Response) IsFileNotExists() bool {
// 	return r.Code == 450
// }

func (r *Response) getTime() (*time.Time, error) {
	var year, month, day, hour, min, sec, nsec int

	// scanf has been taken from the linux ftp
	// client source code.
	parsed, err := fmt.Sscanf(r.Msg,
		"%04d%02d%02d%02d%02d%02d.%03d",
		&year, &month, &day, &hour, &min, &sec, &nsec)
	if err != nil {
		return nil, err
	}
	// if we can't parse nsec is ok
	if parsed == 6 {
		nsec = 0
	}
	if parsed < 6 {
		return nil, fmt.Errorf("Fail to parse date: %s", r.Msg)
	}

	date := time.Date(year, time.Month(month), day, hour, min, sec, nsec, time.UTC)
	return &date, nil
}

func (f *Conn) getFtpResponse() (*Response, error) {
	// response, err := f.responseString()

	//reader := bufio.NewReader(f.control)

	// buf := make([]byte, 1024)
	// n, err := f.control.Read(buf)
	// response := string(buf[:n])

	// buff, _, err := f.controlReader.ReadLine()
	buff, _, err := f.controlRw.ReadLine()

	// fmt.Printf("I read: %d", n)

	//fmt.Fprint(f.config.ResponseFile, response)

	if err != nil {
		return nil, err
	}

	response := string(buff)

	ftpResponse, err := newFtpResponse(response)
	if err != nil {
		return nil, err
	}
	if ftpResponse.IsFtpError() {
		return nil, errors.New(ftpResponse.Error())
	}
	return ftpResponse, nil
}

func (f *Conn) writeCommand(cmd string) error {
	f.controlRw.Flush()

	// we try ascii.
	// src := []byte(cmd)
	// dst := make([]byte, ascii85.MaxEncodedLen(len(src)))

	// ascii85.Encode(dst, src)

	if _, err := f.controlRw.WriteString(cmd); err != nil {
		// if _, err := f.controlRw.Write(dst); err != nil {
		return nil
	}
	err := f.controlRw.Flush()
	return err
}

func (f *Conn) writeCommandAndGetResponse(cmd string) (*Response, error) {
	//if err := f.writeCommand(cmd); err != nil {
	if err := f.writeCommand(cmd); err != nil {
		return nil, err
	}
	return f.getFtpResponse()
}

// newFtpResponse builds a Response object from a string,
// the string should be build in the following way:
// <code> <message>; <message> can be omitted.
func newFtpResponse(response string) (*Response, error) {
	code, err := strconv.Atoi(response[0:3])

	if err != nil {
		return nil, err
	}
	var msg string
	if len(response) < 4 {
		msg = ""
	} else {
		msg = response[4:]
	}
	//msg = strings.TrimSpace(msg)
	// msg = strings.TrimRight(msg, "\r\n")
	// msg = strings.TrimRight(msg, "\r")
	// msg = strings.TrimRight(msg, "\n")

	return &Response{Code: code, Msg: msg}, nil
}

// func inverseResponse(response string) *Response {
// 	code, _ := strconv.Atoi(response[0:3])
// 	msg := response[5:]
//
// 	//msg = strings.TrimSpace(msg)
// 	// msg = strings.TrimRight(msg, "\r\n")
// 	// msg = strings.TrimRight(msg, "\r")
// 	// msg = strings.TrimRight(msg, "\n")
//
// 	return &Response{Code: code, Msg: msg}
// }

func parsePasv(response *Response) (*net.TCPAddr, error) {
	ind := strings.Index(response.Msg, "(")
	if ind == -1 {
		return nil, errors.New("Fail to parse PASV response: '('")
	}
	addr := response.Msg[ind+1 : len(response.Msg)-1]

	members := strings.Split(addr, ",")
	// fmt.Printf("members: %v\n", members)
	if len(members) != 6 {
		return nil, errors.New("Fail to parse PASV response")
	}
	ip := members[0] + "." + members[1] + "." + members[2] + "." + members[3]

	n1Port, err := strconv.Atoi(members[4])
	if err != nil {
		return nil, errors.New("Fail to parse PASV n1 response")
	}
	n2Port, err := strconv.Atoi(members[5])
	if err != nil {
		return nil, errors.New("Fail to parse PASV n2 response")
	}
	port := n1Port*256 + n2Port
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return nil, errors.New("Fail to parse PASV response")
	}
	return &net.TCPAddr{
		IP:   ipAddr,
		Port: port,
	}, nil
}

// CipherSuitesString shows the list of available ciphers.
// If allowWeakHash is set (we strongly suggest to no)
// ciphers with SHA are permitted. We don't permit
// the use of RC4.
func CipherSuitesString(allowWeakHash bool) []string {
	ciphers := []string{
		"tls.TLS_RSA_WITH_AES_128_CBC_SHA256",
		"tls.TLS_RSA_WITH_AES_128_CBC_SHA256",
		"tls.TLS_RSA_WITH_AES_256_GCM_SHA384",
		"tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256",
		"tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256",
		"tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",

		"tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
		"tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",

		"tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256",
		"tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
		"tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
	}
	if allowWeakHash {
		ciphers = append(ciphers,
			"tls.TLS_RSA_WITH_AES_128_CBC_SHA",
			"tls.TLS_RSA_WITH_AES_256_CBC_SHA",
			"tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA",
			"tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
			"tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA",
			"tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA",
		)
	}
	return ciphers
}

func cipherSuites(allowWeakHash bool) []uint16 {
	basicCipherSuites := []uint16{
		tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,

		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,

		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	}
	if allowWeakHash {
		basicCipherSuites = append(basicCipherSuites,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		)
	}
	return basicCipherSuites
}

func (c *Config) initTLS() {
	if c.TLSOption == nil {
		c.TLSOption = &TLSOption{
			AllowSSL:       false,
			ImplicitTLS:    false,
			AllowWeakHash:  false,
			AuthTLSOnFirst: false,
		}
	}

	// if c.TLSOption.AllowSSL || c.TLSOption.ImplicitTLS || c.TLSOption.AuthTLSOnFirst {
	c.tlsConfig = &tls.Config{}
	//}

	if c.TLSOption.AllowSSL {
		c.tlsConfig.MinVersion = tls.VersionSSL30
	} else if c.TLSOption.ImplicitTLS || c.TLSOption.AuthTLSOnFirst {
		if c.tlsConfig.MinVersion <= tls.VersionSSL30 {
			c.tlsConfig.MinVersion = tls.VersionTLS12
		}
	}

	if c.TLSOption.SkipVerify {
		c.tlsConfig.InsecureSkipVerify = true
	} else {
		c.tlsConfig.InsecureSkipVerify = false
	}

	c.tlsConfig.ServerName = c.TLSOption.ServerName

	c.tlsConfig.CipherSuites = cipherSuites(c.TLSOption.AllowWeakHash)
}

func internalDial(remote string, config *Config) (*Conn, *Response, error) {
	var conn net.Conn
	var err error

	dialer := &net.Dialer{
		LocalAddr: &net.TCPAddr{
			IP:   config.LocalIP,
			Port: config.LocalPort,
		},
	}

	if config.DefaultMode == IndMode {
		return nil, nil, errors.New(InvalidMode)
	}

	config.initTLS()

	if config.TLSOption.ImplicitTLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", remote, config.tlsConfig)
	} else {
		conn, err = dialer.Dial("tcp", remote)
	}

	if err != nil {
		return nil, nil, err
	}

	ftpConn := &Conn{
		control:    conn,
		config:     config,
		bufferSize: bufferSize,
	}
	reader, writer := bufio.NewReader(conn), bufio.NewWriter(conn)
	ftpConn.controlRw = bufio.NewReadWriter(reader, writer)

	ftpConn.ctx, ftpConn.cancel = context.WithCancel(context.Background())

	response, err := ftpConn.getFtpResponse()
	if err != nil {
		return nil, nil, err
	}
	if response.Code != FirstConnOk {
		return nil, nil, newUnexpectedCodeError(FirstConnOk, response.Code)
	}

	// always try tls.
	var tlsResponse *Response
	if config.TLSOption.AuthTLSOnFirst {
		tlsResponse, err = ftpConn.AuthTLS(true, true)
		if err != nil {
			return nil, nil, err
		}

		if tlsResponse.Code == NotSupported {
			if config.TLSOption.ContinueIfNoSSL {
				err = errors.New(FailToTLS)
			} else {
				return nil, nil, errors.New(FailToTLS)
			}
		}
	}

	return ftpConn, response, err
}

// Port runs the PORT command on the local IP,
func (f *Conn) port( /*ip net.IP*/ ) (*Response, int, error) {
	// fmt.Printf("hanging trying to get random port")
	port, n1, n2 := f.getRandomPort()
	// fmt.Printf("I got it!")
	// port := n1*265 + n2

	// opening local listener
	// listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", f.config.LocalIp.String(), port))
	// if err != nil {
	// 	return nil, err
	// }
	// f.listenersParams.Enqueue(&port)

	//writing command to the server.
	f.writeCommand("PORT " + portString(f.config.LocalIP, n1, n2) + "\r\n")
	response, err := f.getFtpResponse()
	if err != nil {
		return nil, 0, err
	}

	if response.Code != PortOk {
		return nil, 0, newUnexpectedCodeError(PortOk, response.Code)
	}

	//if ok adding the listener to the listeners list
	// f.listeners.Enqueue(listener)
	return response, port, nil
}

func (f *Conn) openListener() (net.Listener, error) {
	var listener net.Listener
	var err error
	_, port, err := f.port()
	if err != nil {
		return nil, err
	}
	// log.Printf("PORT OK")
	// opening the listener.
	// port := f.listenersParams.Dequeue().(*int)
	listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", f.config.LocalIP.String(), port))
	// log.Printf("Listener Ok")
	if err != nil {
		return nil, err
	}
	return listener, nil
}

func (f *Conn) internalLs(mode Mode, filepath string, doneChan chan<- []string, errChan chan<- error) {

	var receiver io.Reader
	var cmd string

	if filepath == "" {
		cmd = "LIST\r\n"
	} else {
		cmd = "LIST " + filepath + "\r\n"
	}

	if mode == IndMode {
		mode = f.config.DefaultMode
	}

	if mode == ActiveMode {

		// create the listener.
		listener, err := f.openListener()
		if err != nil {
			errChan <- err
			return
		}
		defer listener.Close()

		// sending command.
		response, err := f.writeCommandAndGetResponse(cmd)
		if err != nil {
			errChan <- err
			return
		}

		if response.Code == FileUnavailable {
			errChan <- errors.New(response.String())
			return
		}

		// accept connection
		receiver, err = listener.Accept()
		if err != nil {
			errChan <- err
			return
		}

	} else if mode == PassiveMode {

		addr, err := f.pasvGetAddr()
		if err != nil {
			errChan <- err
			return
		}
		// write command
		if _, err = f.writeCommandAndGetResponse(cmd); err != nil {
			errChan <- err
			return
		}

		receiver, err = f.connectToAddr(addr)
		if err != nil {
			errChan <- err
			return
		}
	}

	buffer := make([]byte, f.bufferSize)
	var result []string

	for {
		_, err := receiver.Read(buffer)
		if err != nil && err == io.EOF {
			break
		} else if err != nil {
			// error
			errChan <- err
			return
		}
		read := string(buffer)
		result = append(result, read)
	}

	// final reading.
	if _, err := f.getFtpResponse(); err != nil {
		errChan <- err
		return
	}

	doneChan <- result
}

func (f *Conn) connectToAddr(addr *net.TCPAddr) (net.Conn, error) {
	return net.Dial("tcp4", addr.String())
}

// func (f *Conn) pasv() (*Response, error) {
// 	return f.writeCommandAndGetResponse("PASV\r\n")
// }

// pasvGetAddr issues the PASV command and then it
// parses the response returning a TCP Addr.
func (f *Conn) pasvGetAddr() (*net.TCPAddr, error) {
	response, err := f.writeCommandAndGetResponse("PASV\r\n")
	if err != nil {
		return nil, err
	}

	if response.Code != PasvOk {
		return nil, newUnexpectedCodeError(PasvOk, response.Code)
	}

	addr, err := parsePasv(response)
	if err != nil {
		return nil, err
	}

	return addr, nil
}

func getPwd(response *Response) (string, error) {
	ind1 := strings.Index(response.Msg, "\"")
	if ind1 == -1 {
		return "", errors.New("Fail to parse response")
	}
	ind2 := strings.LastIndex(response.Msg, "\"")
	if ind2 == -1 {
		return "", errors.New("Fail to parse response")
	}
	directory := response.Msg[ind1+1 : ind2]
	return directory, nil
}

func (f *Conn) internalStore(
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

	if mode == IndMode {
		mode = f.config.DefaultMode
	}

	usedBufferSize := f.bufferSize
	if bufferSize > 0 && bufferSize <= MaxAllowedBufferSize {
		usedBufferSize = bufferSize
	}

	if mode == ActiveMode {

		listener, err := f.openListener()
		if err != nil {
			errChan <- err
			return
		}
		defer listener.Close()

		if _, err = f.writeCommandAndGetResponse("STOR " + dst + "\r\n"); err != nil {
			errChan <- err
			return
		}

		sender, err = listener.Accept()
		if err != nil {
			errChan <- err
			return
		}

	} else if mode == PassiveMode {

		addr, err := f.pasvGetAddr()
		if err != nil {
			errChan <- err
			return
		}

		// write command
		if _, err = f.writeCommandAndGetResponse("STOR " + dst + "\r\n"); err != nil {
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
	file, err := os.Open(src)
	if err != nil {
		errChan <- err
		return
	}

	info, err := file.Stat()
	if err != nil {
		errChan <- err
		return
	}

	buffer := make([]byte, usedBufferSize)

	// command has been issued, notifying on startingChan
	startingChan <- struct{}{}

	for n < int(info.Size()) {

		select {
		case <-f.ctx.Done():
			abortChan <- struct{}{}
		// starting polling on abort
		case <-abortChan:
			// it's not completely correct to close here the data channel,
			// but some server will expect the client to do this.
			sender.Close()
			response, err := f.writeCommandAndGetResponse("ABOR\r\n")
			if err != nil {
				if onEachChan != nil {
					close(onEachChan)
				}
				errChan <- err
				return
			}

			// SOME SERVER LIKE APACHE WILL RETURN US A 226
			// EVEN IF NO FILE HAS BEEN TRANSFERED, WHILE,
			// ACCORDING TO RFC IT'D RETURN US A 426 FOLLOWED BY A 226.
			// SO IT RETURNS US 226 AND 226.

			// this in theory should be only if == AbortOk
			if response.Code == AbortOk || response.Code == TransferOk {
				// after the first response, server must send another with
				// 226.
				abortResponse, err := f.getFtpResponse()
				if err != nil {
					// very bad
					if onEachChan != nil {
						close(onEachChan)
					}
					errChan <- err
					return
				} else if abortResponse.Code != TransferOk {
					if onEachChan != nil {
						close(onEachChan)
					}
					errChan <- newUnexpectedCodeError(TransferOk, abortResponse.Code)
					return
				}
			} else {
				if onEachChan != nil {
					close(onEachChan)
				}
				errChan <- newUnexpectedCodeError(AbortOk, response.Code)
				return
			}

			// deleting the file if required.
			if deleteIfAbort {
				if _, err := f.DeleteFile(dst); err != nil {
					errChan <- err
				}
			}
			if onEachChan != nil {
				close(onEachChan)
			}
			doneChan <- struct{}{}
			return

		// when no abort has been received, go on
		default:
			// reading from file
			read, err := file.Read(buffer)
			if err != nil {
				if onEachChan != nil {
					close(onEachChan)
				}
				sender.Close()
				errChan <- err
				return
			}

			// updating the number of read bytes
			n += read
			_, err = sender.Write(buffer[:read])
			if err != nil {
				if onEachChan != nil {
					close(onEachChan)
				}
				sender.Close()
				errChan <- err
				return
			}
			if onEachChan != nil {
				onEachChan <- read
			}
		}
	}

	// until I close the data connection it doesn't answer me.
	sender.Close()

	// when completed reading response.
	if _, err := f.getFtpResponse(); err != nil {
		if onEachChan != nil {
			close(onEachChan)
		}
		errChan <- err
		return
	}

	doneChan <- struct{}{}
	if onEachChan != nil {
		close(onEachChan)
	}
}

// Retrieve download a file located at filepathSrc to filepathDest.
// When finished, it writes into doneChan. Any error, that'll make it immediately exits,
// is written into errChan.
func (f *Conn) internalRetr(mode Mode,
	filepathSrc,
	filepathDest string,
	doneChan chan<- struct{},
	abortChan chan struct{},
	startingChan chan<- struct{},
	errChan chan<- error,
	onEachChan chan<- int,
	bufferSize int,
) {

	var receiver io.ReadCloser

	if mode == IndMode {
		mode = f.config.DefaultMode
	}

	usedBufferSize := f.bufferSize
	if bufferSize > 0 && bufferSize <= MaxAllowedBufferSize {
		usedBufferSize = bufferSize
	}

	if mode == ActiveMode {

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
	} else if mode == PassiveMode {

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
	buffer := make([]byte, usedBufferSize)

	loop := true

	// command has been issued, notify on startingChan
	startingChan <- struct{}{}

	// filee, _ := os.Create("fuckkk")

	for loop {

		select {
		case <-f.ctx.Done():
			abortChan <- struct{}{}
		case <-abortChan:
			// log.Printf("received abort")
			receiver.Close()
			// var response *Response //declaring here just to prevent go vet.
			response, err := f.writeCommandAndGetResponse("ABOR\r\n")
			if err != nil {
				if onEachChan != nil {
					close(onEachChan)
				}
				errChan <- err

				os.Remove(file.Name()) //skipping the error.
				return
			}

			// if 426 we have to wait for another response.
			// if 226 the transfer is complete
			if response.Code == AbortOk || response.Code == TransferOk {
				// ok, wait for another.
				abortResponse, err := f.getFtpResponse()
				if err != nil {
					if onEachChan != nil {
						close(onEachChan)
					}
					// very bad
					errChan <- err
					return
				} else if abortResponse.Code != TransferOk {
					if onEachChan != nil {
						close(onEachChan)
					}
					errChan <- newUnexpectedCodeError(TransferOk, abortResponse.Code)
					return
				}
			} else {
				if onEachChan != nil {
					close(onEachChan)
				}
				errChan <- newUnexpectedCodeError(AbortOk, response.Code)
				return
			}

			err = os.Remove(file.Name())
			if err != nil {
				errChan <- err
			} else {
				// if onEachChan != nil {
				// 	close(onEachChan)
				// }
				doneChan <- struct{}{}
			}
			if onEachChan != nil {
				close(onEachChan)
			}
			return

		default:
			var n int
			n, err = receiver.Read(buffer)

			if err != nil && err == io.EOF {
				// I know that it is ugly
				// but in this way we can skip some if.
				// EOF means the connection has been closed.
				loop = false
				continue // before there was a break but it'll make
				// exiting just from the select.
			} else if err != nil {
				if onEachChan != nil {
					close(onEachChan)
				}
				receiver.Close()
				errChan <- err
				return
			}

			// so we can skip null bytes that are added
			/// to fill the buffer size.

			// index := bytes.IndexByte(buffer, 0)
			// if index == -1 {
			// 	index = len(buffer)
			// }
			if onEachChan != nil {
				// onEachChan <- struct{}{}

				// onEachChan <- index
				onEachChan <- n
			}

			if _, err = file.Write(buffer[:n]); err != nil {
				// closing the connection as well
				if onEachChan != nil {
					close(onEachChan)
				}
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
		if onEachChan != nil {
			close(onEachChan)
		}
		errChan <- err
		return
	}

	doneChan <- struct{}{}
	if onEachChan != nil {
		close(onEachChan)
	}
}
