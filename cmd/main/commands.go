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

package main

import (
	"fmt"
	"strings"

	"github.com/nbena/ftp"
)

const (
	exitCmd1 = "quit"
	exitCmd2 = "exit"
)

var (
	authSSL = "auth-ssl"
	authTLS = "auth-tls"
	quit    = "quit"
	noop    = "noop"
	pwd     = "pwd"
	cd      = "cd"
	info    = "info"
	ls      = "ls"
	mkdir   = "mkdir"
	mv      = "mv"
	put     = "put"
	get     = "get"
	rm      = "rm"
	setMode = "set-mode"
	getMode = "get-mode"
	help    = "help"

	authSSLHelp = "start an SSL connection"
	authTLSHelp = "start a TLS connection"
	quitHelp    = "exit"
	noopHelp    = "noop, just do nothing"
	pwdHelp     = "show corrent directory"
	cdHelp      = "cd <directory> moving to <directory>"
	infoHelp    = "info <file> show info of <file>, last modification time and size"
	lsHelp      = "ls [directory] ls on [directory] or current directory"
	mkdirHelp   = "mkdir <directory> create a directory"
	mvHelp      = "mv <from> <to>"
	putHelp     = "put <local-file> <remote-destination> upload <local-file> to server using <remote-destination>"
	getHelp     = "get <remote-file> <local-destination> download <remote-file> to <local-destination>"
	rmHelp      = "rm <file> delete remote file/directory"
	setModeHelp = "set-mode active|passive sets the mode to use for the next transfers"
	getModeHelp = "get-mode shows the current use FTP mode"
	helpHelp    = "show this message"

	unrecognizedCmd = "unrecognized command, type 'help' to view a list of available commands, or 'help <cmd>' for specific help"

	helpMap = map[string]*helpEntry{
		authSSL: &helpEntry{
			help:   authSSLHelp,
			isLong: true,
		},
		authTLS: &helpEntry{
			help:   authSSLHelp,
			isLong: true,
		},
		quit: &helpEntry{
			help:   quitHelp,
			isLong: false,
		},
		noop:    &helpEntry{help: noopHelp, isLong: false},
		pwd:     &helpEntry{help: pwdHelp, isLong: false},
		info:    &helpEntry{help: infoHelp, isLong: false},
		ls:      &helpEntry{help: lsHelp, isLong: false},
		mkdir:   &helpEntry{help: mkdirHelp, isLong: false},
		mv:      &helpEntry{help: mvHelp, isLong: false},
		put:     &helpEntry{help: putHelp, isLong: false},
		get:     &helpEntry{help: getHelp, isLong: false},
		rm:      &helpEntry{help: rmHelp, isLong: false},
		help:    &helpEntry{help: helpHelp, isLong: false},
		setMode: &helpEntry{help: setModeHelp, isLong: true},
		getMode: &helpEntry{help: getModeHelp, isLong: true},
		cd:      &helpEntry{help: cdHelp, isLong: false},
	}
)

type helpEntry struct {
	help   string
	isLong bool
}

func (e *helpEntry) String(skipTabs bool) string {
	defaultTab := "\t\t\t"
	if e.isLong {
		defaultTab = "\t\t"
	}
	if skipTabs {
		defaultTab = ""
	}
	return defaultTab + e.help
}

// type ftpFunction func(...interface{}) (*ftp.Response, error)

type cmd struct {
	cmd      string
	args     []string
	required bool
	n        int
	// function ftpFunction
}

func (c *cmd) apply(
	ftpConn *ftp.Conn,
	returnAsString bool,
	args ...interface{},
) (interface{}, error) {
	switch c.cmd {

	// no args
	case authTLS:
		return ftpConn.AuthTLS(allowSSL3)
	case authSSL:
		return ftpConn.AuthSSL()
	case quit:
		return ftpConn.Quit()
	case noop:
		return ftpConn.Noop()
	case pwd:
		_, workingDir, err := ftpConn.Pwd()
		if err != nil {
			return nil, err
		}
		return workingDir, nil

	case getMode:
		mode := ftpConn.Mode()
		return mode, nil

		// 1 arg
	case setMode:
		mode, err := ftp.GetMode(c.args[0])
		if err != nil {
			return nil, err
		}
		ftpConn.SetDefaultMode(mode)
		return nil, nil
	case cd:
		return ftpConn.Cd(c.args[0])
	case info:
		_, size, err := ftpConn.Size(c.args[0])
		if err != nil {
			return nil, err
		}
		_, lastModificationTime, err := ftpConn.LastModificationTime(c.args[0])
		if err != nil {
			return nil, err
		}
		var returnedArray []interface{}
		array := []interface{}{
			size,
			lastModificationTime,
		}
		if returnAsString {
			returnedArray = append(returnedArray, fmt.Sprintf("Size: %d, last modified: %s",
				size,
				lastModificationTime.String()))
		} else {
			returnedArray = array
		}
		return returnedArray, nil
	case ls:
		// doneChan := args[0].(chan []string)
		// errChan := args[1].(chan error)
		var dirs []string
		var err error
		if len(c.args) == 0 {
			// ftpConn.Ls(ftp.IndMode, doneChan, errChan)
			dirs, err = ftpConn.LsSimple(ftp.IndMode)
		} else {
			// ftpConn.LsDir(ftp.IndMode, c.args[0], doneChan, errChan)
			dirs, err = ftpConn.LsDirSimple(ftp.IndMode, c.args[0])
		}

		// nil, nil because returns value are in the channels.
		// return nil, nil
		return dirs, err
	case mkdir:
		return ftpConn.MkDir(c.args[0])

		// 2 arg
	case mv:
		return ftpConn.Rename(c.args[0], c.args[1])

	case put:
		doneChan := args[0].(chan struct{})
		errChan := args[1].(chan error)
		abortChan := args[2].(chan struct{})
		startingChan := args[3].(chan struct{})
		onEachChan := args[4].(chan struct{})

		ftpConn.Store(ftp.IndMode,
			c.args[0],
			c.args[1],
			doneChan,
			abortChan,
			startingChan,
			errChan,
			onEachChan,
			// deleteIfAbort is a global variable.
			deleteIfAbort)
		// return nil, nil

	case get:
		doneChan := args[0].(chan struct{})
		errChan := args[1].(chan error)
		abortChan := args[2].(chan struct{})
		startingChan := args[3].(chan struct{})
		onEachChan := args[4].(chan struct{})
		ftpConn.Retrieve(ftp.IndMode,
			c.args[0],
			c.args[1],
			doneChan,
			abortChan,
			startingChan,
			// delete if abort is not present because it's always done.
			errChan,
			onEachChan,
		)
		// n
	case rm:
		var responses []*ftp.Response
		for _, filename := range c.args {
			// we can't know if it's a file or not,
			// trying some smart 'things'
			var response *ftp.Response
			var err error
			if strings.Contains(filename, ".") {
				// maybe it's a file?
				response, err = ftpConn.DeleteFile(filename)
				if err != nil {
					response, err = ftpConn.DeleteDir(filename)
				}
			} else {
				response, err = ftpConn.DeleteDir(filename)
				if err != nil {
					response, err = ftpConn.DeleteFile(filename)
				}
			}
			if err != nil {
				return nil, err
			}
			responses = append(responses, response)
		}
		return responses, nil
	}
	return nil, nil
}

func parseZeroArg(s string) (*cmd, error) {
	var command cmd
	var err error
	switch s {
	case authTLS:
		command = commandAuthTLS
	case authSSL:
		command = commandAuthSSL
	case quit:
		command = commandQuit
	case noop:
		command = commandNoop
	case pwd:
		command = commandPwd
	case ls:
		command = commandLs
	case getMode:
		command = commandGetMode
	default:
		err = fmt.Errorf("Unknown command or wrong parameters: %s", s)
	}
	return &command, err
}

func parseOneArg(first, second string) (*cmd, error) {
	var command cmd
	var err error
	switch first {
	case cd:
		command = commandCd
	case info:
		command = commandFileInfo
	case mkdir:
		command = commandMkdir
	case ls:
		command = commandLs
	case rm:
		command = commandRm
	case setMode:
		command = commandSetMode
	default:
		err = fmt.Errorf("Unknown command or wrong parameters: %s", first)
	}
	if err == nil {
		command.args = []string{second}
	}
	return &command, err
}

func parseTwoArg(first, second, third string) (*cmd, error) {
	var command cmd
	var err error
	switch first {
	case mv:
		command = commandRename
	// case rm:
	// 	command = commandRm
	case get:
		command = commandGet
	case put:
		command = commandPut
	default:
		err = fmt.Errorf("Unknown command or wrong parameters: %s", first)
	}
	if err == nil {
		command.args = []string{second, third}
	}
	return &command, err
}

// func parseNArg(first string, others []string) (*cmd, error) {
// 	var command cmd
// 	var err error
// 	switch first {
// 	case rm:
// 		command = commandRm
// 	case get:
// 		command = commandGet
// 		if len(others) != 2 {
// 			err = fmt.Errorf("Wrong args length for 'get': %d, help: %s", len(others), helpMap[get])
// 		}
// 	case put:
// 		command = commandPut
// 		if len(others) != 2 {
// 			err = fmt.Errorf("Wrong args length for 'put': %d, help: %s", len(others), helpMap[put])
// 		}
// 	default:
// 		err = fmt.Errorf("Unknown command n: %s", first)
// 	}
// 	if err == nil {
// 		command.args = others
// 	}
// 	return &command, err
// }

func parse(s string) (*cmd, error) {
	var cmd *cmd
	var err error
	if strings.LastIndex(s, " ") == -1 {
		cmd, err = parseZeroArg(s)
	} else if strings.Count(s, " ") == 1 {
		parsed := strings.Split(s, " ")
		cmd, err = parseOneArg(parsed[0], parsed[1])
	} else if strings.Count(s, " ") == 2 {
		parsed := strings.Split(s, " ")
		cmd, err = parseTwoArg(parsed[0], parsed[1], parsed[2])
	} else {
		// parsed := strings.Split(s, " ")
		// if len(parsed) <= 2 {
		// 	err = fmt.Errorf("Fail to parse command: %s", s)
		// } else {
		// 	cmd, err = parseNArg(parsed[0], parsed[1:])
		// }
		err = fmt.Errorf("Unknown command or wrong parameters: %s", s)
	}
	if err != nil {
		return nil, err
	}
	return cmd, err
}

func parseAllCommands(s string) ([]*cmd, error) {
	parsed := strings.Split(s, ";")
	var cmds []*cmd
	var command *cmd
	var err error
	if len(parsed) == 0 {
		cmds = []*cmd{}
	} else if len(parsed) == 1 {
		command, err = parse(parsed[0])
		if err != nil {
			cmds = []*cmd{command}
		}
	} else {
		cmds = make([]*cmd, len(parsed))
		for i, v := range parsed {
			command, err = parse(v)
			if err != nil {
				cmds[i] = command
			} else {
				break
			}
		}
	}
	return cmds, err
}

var (
	commandAuthTLS = cmd{
		cmd:      "auth-tls",
		required: false,
		n:        0,
		// function: ftpFunction(ftp.AuthTLS()),
	}
	commandAuthSSL = cmd{
		cmd:      "auth-ssl",
		required: false,
		n:        0,
	}
	// commandAbort = cmd{
	// 	cmd:      "abort",
	// 	required: false,
	// 	n:        1,
	// }
	commandCd = cmd{
		cmd:      "cd",
		required: true,
		n:        1,
	}
	commandRm = cmd{
		cmd:      "rm",
		required: true,
		n:        1,
	}
	commandLastMod = cmd{
		cmd:      "last-mod",
		required: true,
		n:        1,
	}
	// commandFileInfo issues size and mdtm.
	commandFileInfo = cmd{
		cmd:      "info",
		required: true,
		n:        1,
	}
	commandLs = cmd{
		cmd:      "ls",
		required: true,
		n:        1,
	}
	commandMkdir = cmd{
		cmd:      "mkdir",
		required: true,
		n:        1,
	}
	commandNoop = cmd{
		cmd:      "noop",
		required: false,
		n:        0,
	}
	commandPwd = cmd{
		cmd:      "pwd",
		required: false,
		n:        0,
	}
	commandQuit = cmd{
		cmd:      "quit",
		required: false,
		n:        0,
	}
	commandRename = cmd{
		cmd:      "mv",
		required: true,
		n:        2,
	}
	commandGet = cmd{
		cmd:      "get",
		required: true,
		n:        -1,
	}
	commandPut = cmd{
		cmd:      "put",
		required: true,
		n:        -1,
	}
	commandSetMode = cmd{
		cmd:      "set-mode",
		required: true,
		n:        1,
	}
	commandGetMode = cmd{
		cmd:      "get-mode",
		required: false,
		n:        0,
	}

	// commandsTable map[string]cmd
	// longCommands  []string
)

// func init() {
// 	commandsTable = make(map[string]cmd)
// 	commandsTable[commandAuthTLS.cmd] = commandAuthTLS
// 	commandsTable[commandAuthSSL.cmd] = commandAuthSSL
// 	commandsTable[commandAbort.cmd] = commandAbort
// 	commandsTable[commandCd.cmd] = commandCd
// 	commandsTable[commandRm.cmd] = commandRm
// 	commandsTable[commandFileInfo.cmd] = commandFileInfo
// 	commandsTable[commandLs.cmd] = commandLs
// 	commandsTable[commandGet.cmd] = commandGet
// 	commandsTable[commandPut.cmd] = commandPut
// 	commandsTable[commandPwd.cmd] = commandPwd
// 	commandsTable[commandQuit.cmd] = commandQuit
// 	commandsTable[commandMkdir.cmd] = commandMkdir
// 	commandsTable[commandNoop.cmd] = commandNoop
// 	commandsTable[commandRename.cmd] = commandRename
//
// 	longCommands = []string{
// 		commandAuthSSL.cmd,
// 		commandAuthTLS.cmd,
// 		commandSetMode.cmd,
// 		commandGetMode.cmd,
//	}
// }
