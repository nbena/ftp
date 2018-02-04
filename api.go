package ftp

import (
	"bytes"
	"crypto/tls"
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
	// FTP_ACTIVE means that for default the
	// active modality will be used.
	FTP_MODE_ACTIVE = Mode(1)
	// FTP_PASSIVE means that the passive mode
	// will be used.
	FTP_MODE_PASSIVE = Mode(2)

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

// func GetReadWriterFromConn(conn net.Conn) *bufio.ReadWriter {
// 	readConn := bufio.NewReader(conn)
// 	writeConn := bufio.NewWriter(conn)
// 	return bufio.NewReadWriter(readConn, writeConn)
// }

// IsFtpError returns true if the response represents
// an error. That means that the code is >=500 && < 600.
func (r *Response) IsFtpError() bool {
	return r.Code >= 500 && r.Code < 600
}

// func (f *Conn) responseString() (string, error) {
//
// 	buf := make([]byte, 1024)
// 	_, err := f.control.Read(buf)
// 	return string(buf), err
// }

// Dial connects to the ftp server.
func Dial(remote string, config *Config) (*Conn, *Response, error) {
	// var conn net.Conn
	// var ip net.IP
	// var err error
	//
	// addr := strings.Split(remote, ":")
	// if len(addr) != 2 {
	// 	return nil, errors.New("Remote must be ip/hostname:port")
	// }
	//
	// remoteAddr := addr[0]
	// port, err := strconv.Atoi(addr[1])
	// if err != nil {
	// 	return nil, err
	// }
	//
	// //tryng to parse IP.
	// ip = net.ParseIP(remoteAddr)
	// if ip != nil {
	// 	// looking up
	// 	addresses, err := net.LookupIP(remoteAddr)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	ip = addresses[0]
	// }

	return internalDial(remote, config)
}

// func ConnectTo(uri string, config *Config) (*Conn, error) {
//
// 	var conn net.Conn
// 	var err error
//
// 	if config.TLSConfig == nil {
// 		conn, err = net.Dial("tcp", uri)
// 	} else {
// 		conn, err = tls.Dial("tcp", uri, config.TLSConfig)
// 	}
//
// 	if err != nil {
// 		return nil, err
// 	}
// 	connection := &Conn{
// 		control:   conn,
// 		config:    config,
// 		listeners: lane.NewStack(),
// 		data:      lane.NewStack(),
// 		rand:      rand.New(rand.NewSource(time.Now().UnixNano())),
// 	}
//
// 	_, err = connection.getFtpResponse()
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	return connection, nil
// }

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

// func (f *Conn) openPortConnection(ip net.IP, n1, n2 int) (net.Listener, error) {
// 	port := n1*256 + n2
// 	return net.Listen("tcp", ":"+strconv.Itoa(port))
// }
//
// func (f *Conn) portInit(ip net.IP, n1, n2 int) (*FtpResponse, error) {
//
// 	listener, err := f.openPortConnection(ip, n1, n2)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	f.writeCommand("PORT " + PortString(ip, n1, n2) + "\r\n")
// 	response, err := f.GetFtpResponse()
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	//once is done add the new listner to the listeners' stack.
// 	f.listeners.Push(listener)
//
// 	//if ok Accept a new connection.
//
// 	return response, nil
// }

// func (f *Conn) portAccept() error {
// 	if f.listeners.Empty() {
// 		return errors.New("A new listener must be created")
// 	}
//
// 	//use type assertion
// 	listener := f.listeners.Pop().(net.Listener)
//
// 	conn, err := listener.Accept()
// 	if err != nil {
// 		return err
// 	}
//
// 	f.data.Enqueue(conn)
// 	return nil
// }

//Port execute a PORT command to the FTP Server. The first param is the IP
//address which the server should connect to, the second is the port.
//Plase notiche that the port is 'complete'.
//The port will be sequently divided into the 2 numbers required by the FTP
//protocol.
// func (f *Conn) Port(ip net.IP, port int) (*Response, error) {
// 	n1, n2 := PortNumbers(port)
// 	return f.portInit(ip, n1, n2)
// }

//Quit close the current FTP session, it means that every transfer in progress
//is closed too.
func (f *Conn) Quit() (*Response, error) {
	response, err := f.writeCommandAndGetResponse("QUIT\r\n")
	if err != nil {
		return nil, err
	}
	//for _, dataConn := range f.data {
	//	err = dataConn.Close()
	//}

	// for !f.data.Empty() {
	// 	dataConn := f.data.Pop().(net.Conn)
	// 	dataConn.Close()
	// }
	//
	// for !f.listeners.Empty() {
	// 	listener := f.listeners.Pop().(net.Listener)
	// 	listener.Close()
	// }

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

		_, err = f.writeCommandAndGetResponse("STOR " + fileName + "\r\n")
		if err != nil {
			errChan <- err
			return
		}

		sender, err = listener.Accept()
		if err != nil {
			errChan <- err
			return
		}

	} else {

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
		_, err = sender.Write(buffer)
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
	if err := f.writeCommand("DELE " + filepath + "\r\n"); err != nil {
		return nil, err
	}
	return f.getFtpResponse()
}

// MkDir creates a directory.
func (f *Conn) MkDir(name string) (*Response, error) {
	if err := f.writeCommand("MKD " + name + "\r\n"); err != nil {
		return nil, err
	}
	return f.getFtpResponse()
}

// DeleteDir deletes a directory.
func (f *Conn) DeleteDir(name string) (*Response, error) {
	if err := f.writeCommand("RMD " + name + "\r\n"); err != nil {
		return nil, err
	}
	return f.getFtpResponse()
}

// Cd change the working directory.
func (f *Conn) Cd(path string) (*Response, error) {
	if err := f.writeCommand("CWD " + path + "\r\n"); err != nil {
		return nil, err
	}
	return f.getFtpResponse()
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
	} else {

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

// PutGo sends the file 'path' to the server. Mode specifies if it is
// necessary to use active or passive. Done will be written when stuff
// is finished or when an error happens. If everything is fine an empty
// string will be written. In future two channels can be added.
// func (f *Conn) PutGo(filepath string, mode *Mode, done chan<- string) {
//
// 	if mode == nil {
// 		mode = &f.config.DefaultMode
// 	}
//
// 	_, fileName := path.Split(filepath)
//
// 	listener, err := f.openListenerConnection(*mode)
// 	// log.Printf("ret ok")
// 	if err != nil {
// 		// log.Printf("error here: %s", err.Error())
// 		done <- err.Error()
// 		return
// 	}
// 	defer listener.Close()
// 	// log.Printf("defer ok")
//
// 	// log.Printf("Ok going to send command")
//
// 	err = f.writeCommand("STOR " + fileName + "\r\n")
// 	if err != nil {
// 		done <- err.Error()
// 		return
// 	}
//
// 	// log.Printf("Waiting response")
// 	_, err = f.getFtpResponse()
// 	if err != nil {
// 		done <- err.Error()
// 		return
// 	}
// 	// log.Printf("After STOU: %s", response.String())
//
// 	// log.Printf("Going to accept")
// 	sender, err := listener.Accept()
// 	if err != nil {
// 		done <- err.Error()
// 		return
// 	}
//
// 	// log.Printf("Accepted")
//
// 	// var buf []byte
// 	var n int64
// 	file, err := os.Open(filepath)
// 	if err != nil {
// 		done <- err.Error()
// 		return
// 	}
//
// 	info, err := file.Stat()
// 	if err != nil {
// 		done <- err.Error()
// 		return
// 	}
// 	// if info.Size() < 2048 {
// 	// 	buf = make([]byte, info.Size())
// 	// } else {
// 	// 	buf = make([]byte, 2048)
// 	// }
//
// 	// the file reader puts bytes here
// 	sendChan := make(chan []byte)
// 	// the errchan is used by the file reader to notify errors
// 	// to the sender.
// 	errChan := make(chan error)
// 	// the deletechan is used by the sending to notify errors
// 	// to the file reader.
// 	deleteChan := make(chan struct{})
// 	// this channel is used by the file reader to notify it has
// 	// done.
// 	doneChan := make(chan struct{})
//
// 	// reader
// 	go func() {
// 		buf := make([]byte, 1024)
// 		// while there is something to read
// 		for n < info.Size() {
// 			select {
// 			// if the other notifies me of an error
// 			// just exit.
// 			case <-deleteChan:
// 				return
// 			default:
// 				// none reading.
// 				read, err := file.Read(buf)
// 				n = int64(read)
// 				if err != nil {
// 					// if an error occurs I notify it, then
// 					// exit.
// 					errChan <- err
// 					return
// 				}
// 				// sending data.
// 				sendChan <- buf
// 			}
// 		}
// 		// notify I'm done.
// 		doneChan <- struct{}{}
// 		close(sendChan)
// 	}()
//
// 	// sender
// 	for {
// 		select {
// 		// if I get notified of an error
// 		// I notify it, then I exit.
// 		case err := <-errChan:
// 			// tell the other to quit
// 			// deleteChan <- struct{}{}
// 			sender.Close()
// 			done <- err.Error()
// 			return
// 		// if data I put on the connection.
// 		case data := <-sendChan:
// 			n, err := sender.Write(data)
// 			// if errors, notify to the reader and exit.
// 			if err != nil {
// 				deleteChan <- struct{}{}
// 				sender.Close()
//
// 				done <- err.Error()
// 				return
// 			}
// 			if n != len(data) {
// 				deleteChan <- struct{}{}
// 				sender.Close()
// 				done <- fmt.Sprintf("Partial write: want %d, have %d", len(data), n)
// 				return
// 			}
// 			// the reader tells me it has done.
// 		case <-doneChan:
// 			// log.Printf("Ok sent file %s", filepath)
// 			sender.Close()
//
// 			//now receiveng respose
// 			_, err := f.getFtpResponse()
// 			if err != nil {
// 				done <- err.Error()
// 			} else {
// 				// if here everying is fine.
// 				done <- ""
// 			}
// 			return
// 		}
// 	}
// }
