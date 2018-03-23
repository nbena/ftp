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
	"bytes"
	"io"
	"os"
)

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

	var sender io.WriteCloser

	// _, fileName := path.Split(src)

	if mode == FTP_MODE_ACTIVE {

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

	} else if mode == FTP_MODE_PASSIVE {

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

	buffer := make([]byte, 1024)

	// command has been issued, notifying on startingChan
	startingChan <- struct{}{}

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

			// SOME SERVER SUCH APACHE WILL RETURN US A 226
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
					errChan <- err
					return
				} else if abortResponse.Code != TransferOk {
					errChan <- newUnexpectedCodeError(TransferOk, abortResponse.Code)
					return
				}
			} else {
				errChan <- newUnexpectedCodeError(AbortOk, response.Code)
				return
			}

			// deleting the file if required.
			if deleteIfAbort {
				if _, err := f.DeleteFile(dst); err != nil {
					errChan <- err
				}
			}
			doneChan <- struct{}{}
			return

		// when no abort has been received, go on
		default:
			// reading from file
			read, err := file.Read(buffer)
			if err != nil {
				sender.Close()
				errChan <- err
				return
			}

			// updating the number of read bytes
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

// Retrieve download a file located at filepathSrc to filepathDest.
// When finished, it writes into doneChan. Any error, that'll make it immediately exits,
// is written into errChan.
func (f *Conn) Retrieve(mode Mode,
	filepathSrc,
	filepathDest string,
	doneChan chan<- struct{},
	abortChan <-chan struct{},
	startingChan chan<- struct{},
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

	// command has been issued, notify on startingChan
	startingChan <- struct{}{}

	for loop {

		select {

		case <-abortChan:
			receiver.Close()
			// var response *Response //declaring here just to prevent go vet.
			response, err := f.writeCommandAndGetResponse("ABOR\r\n")
			if err != nil {
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
					// very bad
					errChan <- err
					return
				} else if abortResponse.Code != TransferOk {
					errChan <- newUnexpectedCodeError(TransferOk, abortResponse.Code)
					return
				}
			} else {
				errChan <- newUnexpectedCodeError(AbortOk, response.Code)
				return
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
