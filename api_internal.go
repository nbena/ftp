package ftp

import (
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

	buf := make([]byte, 1024)
	_, err := f.control.Read(buf)
	response := string(buf)

	//fmt.Fprint(f.config.ResponseFile, response)

	if err != nil {
		return nil, err
	}

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
	//fmt.Print(response)
	code, _ := strconv.Atoi(response[0:3])
	msg := response[4:]
	return &Response{Code: code, Msg: msg}
}

func parsePasv(response *Response) (*net.TCPAddr, error) {
	ind := strings.Index(response.Msg, "(")
	if ind == -1 {
		return nil, errors.New("Fail to parse PASV response")
	}
	addr := response.Msg[ind+1 : len(response.Msg)-2]
	// fmt.Printf("addr: %s\n", addr)
	// var n1, n2 string
	// ip := ""
	// cont := 0
	// for i := 0; i < len(addr); i++ {
	// 	if cont < 4 {
	// 		ip += string(addr[i])
	// 	}
	// 	if addr == "," {
	// 		cont++
	// 	}
	// 	if cont == 4 {
	// 		n1 += string(addr[i])
	// 	} else if cont == 5 {
	// 		n2 += string(addr[i])
	// 	}
	// }
	members := strings.Split(addr, ",")
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
	port := n1Port*7 + n2Port
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
		lastUsedMod: FTP_MODE_IND,
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
		_, err = f.writeCommandAndGetResponse(cmd)
		if err != nil {
			errChan <- err
			return
		}

		// accept connection
		receiver, err = listener.Accept()
		if err != nil {
			errChan <- err
			return
		}

	} else {

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
