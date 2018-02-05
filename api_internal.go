package ftp

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

func (f *Conn) getFtpResponse() (*Response, error) {
	// response, err := f.responseString()

	//reader := bufio.NewReader(f.control)

	// buf := make([]byte, 1024)
	// n, err := f.control.Read(buf)
	// response := string(buf[:n])

	buff, _, err := f.controlReader.ReadLine()

	// fmt.Printf("I read: %d", n)

	//fmt.Fprint(f.config.ResponseFile, response)

	if err != nil {
		return nil, err
	}

	response := string(buff)

	ftpResponse := newFtpResponse(response)
	if ftpResponse.IsFtpError() {
		return nil, errors.New(ftpResponse.Error())
	}

	return ftpResponse, nil
}

func (f *Conn) writeCommand(cmd string) error {
	_, err := f.control.Write([]byte(cmd))
	return err
}

func (f *Conn) writeCommandAndGetResponse(cmd string) (*Response, error) {
	if err := f.writeCommand(cmd); err != nil {
		return nil, err
	}
	return f.getFtpResponse()
}

func newFtpResponse(response string) *Response {
	code, _ := strconv.Atoi(response[0:3])
	msg := response[4:]

	//msg = strings.TrimSpace(msg)
	// msg = strings.TrimRight(msg, "\r\n")
	// msg = strings.TrimRight(msg, "\r")
	// msg = strings.TrimRight(msg, "\n")

	return &Response{Code: code, Msg: msg}
}

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

func internalDial(remote string, config *Config) (*Conn, *Response, error) {
	var conn net.Conn
	var err error

	dialer := &net.Dialer{
		LocalAddr: &net.TCPAddr{
			IP:   config.LocalIP,
			Port: config.LocalPort,
		},
	}

	if config.TLSConfig == nil {
		conn, err = dialer.Dial("tcp", remote)
	} else {
		conn, err = tls.DialWithDialer(dialer, "tcp", remote, config.TLSConfig)
	}

	if err != nil {
		return nil, nil, err
	}

	ftpConn := &Conn{
		control: conn,
		config:  config,
		// listenersParams: lane.NewQueue(),
		// listeners:       lane.NewQueue(),
		// data:            lane.NewQueue(),
		lastUsedMod:   FTP_MODE_IND,
		controlReader: bufio.NewReader(conn),
		// rand:            rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	response, err := ftpConn.getFtpResponse()
	if err != nil {
		return nil, nil, err
	}

	return ftpConn, response, nil
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

	if mode == FTP_MODE_ACTIVE {
		// create the listener.
		listener, err := f.openListener()
		if err != nil {
			errChan <- err
			return
		}
		defer listener.Close()

		// sending command.
		if _, err = f.writeCommandAndGetResponse(cmd); err != nil {
			errChan <- err
			return
		}

		// accept connection
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

	buffer := make([]byte, 1024)
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
		// log.Printf(read + "\n")
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

func (f *Conn) pasv() (*Response, error) {
	return f.writeCommandAndGetResponse("PASV\r\n")
}

func (f *Conn) pasvGetAddr() (*net.TCPAddr, error) {
	response, err := f.pasv()
	if err != nil {
		return nil, err
	}

	addr, err := parsePasv(response)
	if err != nil {
		return nil, err
	}

	return addr, nil
}

// func (f *Conn) pasvAndConnect() (net.Conn, error) {
// 	response, err := f.pasv()
// 	if err != nil {
// 		return nil, err
// 	}
// 	addr, err := parsePasv(response)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	return f.getPasvConnection(addr)
// }
