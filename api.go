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
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
)

// Mode is used as a constant
//to specify which type of
//default ftp mode should be used
//(active or passive).
type Mode int

const (
	// MaxAllowedBufferSize is the maximum buffer size that
	// can be set when down/uploading a file, so developers
	// CAN'T require more then 3M of buffer
	MaxAllowedBufferSize = 1024 * 1024 * 5

	// ActiveMode means that for default the
	// active modality will be used.
	ActiveMode = Mode(1)

	// PassiveMode means that the passive mode
	// will be used.
	PassiveMode = Mode(2)

	// IndMode implements this.
	IndMode = Mode(0)

	// AlreadyTLS is the error (error with this content)
	// that is reported everytime an auth tls/ssl is issued
	// on an already tls-ed-connection.
	AlreadyTLS = "The control connection is already SSL or TLS"

	// FailToTLS is the error msg returned in case no support for
	// SSL and TLS has been found.
	FailToTLS = "The server doesn't support neither SSL or TLS"

	// AbortOk is the expected return code for an ABORT code.
	AbortOk = 426

	// CdOk is the expected return code for a CWD.
	CdOk = 250

	// DeleteFileOk is the expected return code when a file has been removed.
	DeleteFileOk = 250

	// DeleteDirOk is the expected return code when a file has been removed.
	DeleteDirOk = 250

	// FileUnavailable is the return code when a file doesn't exists/is busy
	// or something similar.
	FileUnavailable = 450

	// FirstConnOk is what server writes when a connection occured.
	FirstConnOk = 220

	// LastModificationTimeOk is the expected returned code for
	// MDTM command.
	// see https://tools.ietf.org/html/rfc3659#page-8
	LastModificationTimeOk = 213

	// LoginOk is the expected return code for a PASS command.
	LoginOk = 230

	// MkDirOk is the expected return code for a MKD command.
	MkDirOk = 257

	// NoopOk is the expected return code for a NOOP command.
	NoopOk = 200

	// NotSupported is the return code when the server doesn't support
	// the feature/command/requested.
	NotSupported = 431

	// PasvOk is the expected return code for a PASV command.
	PasvOk = 227

	// PortOk is the expected return code for a PORT command.
	PortOk = 200

	// PwdOk is the expected return code for a PWD command.
	PwdOk = 257

	// QuitOk is the expected return code for a QUIT command.
	QuitOk = 221

	// SizeOk is the expected returned code for a SIZE command.
	// see https://tools.ietf.org/html/rfc3659#page-11
	SizeOk = 213

	// TransferOk is the expected returned code received upon
	// a transfer completition.
	TransferOk = 226

	// UsernameOk is the expected return code for a USER command.
	UsernameOk = 331

	// InvalidMode is the error msg returned when default Mode is passed
	// and it is not allowed.
	InvalidMode = "Invalid Mode, only ActiveMode and ModePassive are allowed"

	// ActiveModeStr is the FTP mode active.
	ActiveModeStr = "active"

	// PassiveModeStr is the FTP mode passive.
	PassiveModeStr = "passive"

	// DefaultModeStr is a 'no-matters' FTP mode.
	DefaultModeStr = "default"

	bufferSize = 1024
)

// UnexpectedCodeError is the type that repesents an error that
// occuts when server returns us a code different from the expected.
type UnexpectedCodeError struct {
	Expected int
	Got      int
}

func newUnexpectedCodeError(expected, got int) error {
	return &UnexpectedCodeError{
		Expected: expected,
		Got:      got,
	}
}

func (e *UnexpectedCodeError) Error() string {
	return fmt.Sprintf("unxpected code, want %d, got %d",
		e.Expected,
		e.Got)
}

func unexpectedErrorOrResponse(expected int, response *Response) (*Response, error) {
	if response.Code != expected {
		return nil, newUnexpectedCodeError(expected, response.Code)
	}
	return response, nil
}

// Config contains the
//parameter used for the connection.
type Config struct {
	//The default modality (active or passive).
	DefaultMode Mode

	//Where to put response from the server.
	//Usually it is set to /dev/null or os.Stdin.
	// ResponseFile *os.File
	//Optionally, the tls configuration to use for
	//the connection. This is required only if
	// you think TLS will be used. Note that the library
	// will overwrite some parameters, including MinVersion,
	// according to AllowSSL. Only version 3.0  is allowed. If SSL
	// is not allowed, the MinVersion will be set to TLS 1.2,
	// except if you set another version gt SSL3 lte TLS 1.2.
	// This is done in the Dial function.
	tlsConfig *tls.Config
	TLSOption *TLSOption
	LocalIP   net.IP
	LocalPort int
	Username  string
	Password  string
	FirstPort int
}

// TLSOption is the struct passed to configure TLS params.
type TLSOption struct {
	// If set to true, the first thing that the client
	// will do will be a TLS handshake. If it fails,
	// it'll issue an AUTH SSL/TLS.
	ImplicitTLS bool
	// If set to true, the first thing that the client
	// will do will be an AUTH SSL/TLS.
	// First, TLS, then SSL 3 if TLS is not supported.
	AuthTLSOnFirst bool
	// True allows SSL3.
	AllowSSL bool
	// Whether is set to true it allows to
	// continue operation if no SSL/TLS is supported.
	ContinueIfNoSSL bool
	// If set to true, the list of ciphersuites
	// will include the following algorithms:
	// TLS_RSA_WITH_AES_128_CBC_SHA
	// TLS_RSA_WITH_AES_256_CBC_SHA
	AllowWeakHash bool
	// same value of tls.Config.InsecureSkipVerify
	SkipVerify bool
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
	// rand   *rand.Rand
	bufferSize int

	// These two are used to implement graceful shudtown.
	// When we a used calls quit, the cancel function is called,
	// causing the internal context's channel to send a value,
	// and functions that internally uses abortChannels basically listen
	// on ctx.Done() too.
	ctx    context.Context
	cancel context.CancelFunc
}

// SetDefaultMode sets the FTP mode to be used for the next requests.
// This is used only when functions that takes a Mode type param have that
// param set to 'ModeInd'. If so, this provided mode will be used. If no,
// the mode provided in that function will be used.
func (f *Conn) SetDefaultMode(mode Mode) error {
	if mode == IndMode {
		return errors.New(InvalidMode)
	}
	f.config.DefaultMode = mode
	return nil
}

// GetMode returns the FTP mode from the mode param.
func GetMode(mode string) (Mode, error) {
	var ftpMode Mode
	if mode == ActiveModeStr {
		ftpMode = ActiveMode
	} else if mode == PassiveModeStr {
		ftpMode = PassiveMode
	} else if mode == DefaultModeStr {
		ftpMode = IndMode
	} else {
		return -1, errors.New(InvalidMode)
	}
	return ftpMode, nil
}

// Mode returns the current default modality.
func (f *Conn) Mode() string {
	// var mode string
	// if f.config.DefaultMode == ActiveMode {
	// 	mode = "active"
	// } else if f.config.DefaultMode == ActiveMode {
	// 	mode = "passive"
	// } else {
	// 	mode = "default"
	// }
	return modeStr(f.config.DefaultMode)
}

func modeStr(mode Mode) string {
	var modeStr string
	if mode == ActiveMode {
		modeStr = "active"
	} else if mode == PassiveMode {
		modeStr = "passive"
	} else {
		modeStr = "default"
	}
	return modeStr
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

// BufferSize returns the buffer size used when down/up-loading files,
// which is 1024 bytes.
func (f *Conn) BufferSize() int {
	return bufferSize
}
