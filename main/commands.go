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

type ftpFunction func(...interface{}) (*ftp.Response, error)

type cmd struct {
	cmd      string
	args     []string
	required bool
	n        int
	function ftpFunction
}

func (c *cmd) apply(ftpConn *ftp.Conn, args ...interface{}) (interface{}, error) {
	switch c.cmd {

	// no args
	case "auth-tls":
		return ftpConn.AuthTLS(allowSSL3)
	case "auth-ssl":
		return ftpConn.AuthSSL()
	case "quit":
		return ftpConn.Quit()
	case "noop":
		return ftpConn.Noop()
	case "pwd":
		_, pwd, err := ftpConn.Pwd()
		if err != nil {
			return nil, err
		}
		return pwd, nil

		// 1 arg
	case "cd":
		return ftpConn.Cd(c.args[0])
	case "info":
		_, size, err := ftpConn.Size(c.args[0])
		if err != nil {
			return nil, err
		}
		_, lastModificationTime, err := ftpConn.LastModificationTime(c.args[0])
		if err != nil {
			return nil, err
		}
		return []interface{}{
			size,
			lastModificationTime,
		}, nil
	case "ls":
		doneChan := args[0].(chan<- []string)
		errChan := args[1].(chan<- error)
		ftpConn.LsDir(ftp.FTP_MODE_IND, c.args[0], doneChan, errChan)
		// nil, nil because returns value are in the channels.
		return nil, nil
	case "mkdir":
		return ftpConn.MkDir(c.args[0])

		// 2 arg
	case "mv":
		return ftpConn.Rename(c.args[0], c.args[1])
	case "rm":
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

			}
		}
		return responses, nil
	}
	return nil, nil
}

func parseZeroArg(s string) (*cmd, error) {
	var command cmd
	var err error
	switch s {
	case "auth-tls":
		command = CommandAuthTLS
	case "auth-ssl":
		command = CommandAuthSSL
	case "quit":
		command = CommandQuit
	case "noop":
		command = CommandNoop
	case "pwd":
		command = CommandPwd
	default:
		err = fmt.Errorf("Unknown command: %s", s)
	}
	return &command, err
}

func parseOneArg(first, second string) (*cmd, error) {
	var command cmd
	var err error
	switch first {
	case "cd":
		command = CommandCd
	case "info":
		command = CommandFileInfo
	case "ls":
		command = CommandLs
	case "mkdir":
		command = CommandMkdir
	default:
		err = fmt.Errorf("Unknown command: %s", first)
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
	case "mv":
		command = CommandRename
	default:
		err = fmt.Errorf("Unknown command: %s", first)
	}
	if err != nil {
		command.args = []string{second, third}
	}
	return &command, err
}

func parseNArg(first string, others []string) (*cmd, error) {
	var command cmd
	var err error
	switch first {
	case "rm":
		command = CommandRm
	case "get":
		command = CommandGet
	case "put":
		command = CommandPut
	default:
		err = fmt.Errorf("Unknown command: %s", first)
	}
	if err != nil {
		command.args = others
	}
	return &command, err
}

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
		parsed := strings.Split(s, " ")
		if len(parsed) <= 2 {
			err = fmt.Errorf("Fail to parse command: %s", s)
		} else {
			cmd, err = parseNArg(parsed[0], parsed[1:])
		}
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
	CommandAuthTLS = cmd{
		cmd:      "auth-tls",
		required: false,
		n:        0,
		// function: ftpFunction(ftp.AuthTLS()),
	}
	CommandAuthSSL = cmd{
		cmd:      "auth-ssl",
		required: false,
		n:        0,
	}
	CommandAbort = cmd{
		cmd:      "abort",
		required: false,
		n:        1,
	}
	CommandCd = cmd{
		cmd:      "cmd",
		required: true,
		n:        1,
	}
	CommandRm = cmd{
		cmd:      "rm",
		required: true,
		n:        -1,
	}
	CommandLastMod = cmd{
		cmd:      "last-mod",
		required: true,
		n:        1,
	}
	// CommandFileInfo issues size and mdtm.
	CommandFileInfo = cmd{
		cmd:      "info",
		required: true,
		n:        1,
	}
	CommandLs = cmd{
		cmd:      "ls",
		required: true,
		n:        1,
	}
	CommandMkdir = cmd{
		cmd:      "mkdir",
		required: true,
		n:        1,
	}
	CommandNoop = cmd{
		cmd:      "noop",
		required: false,
		n:        0,
	}
	CommandPwd = cmd{
		cmd:      "pwd",
		required: false,
		n:        0,
	}
	CommandQuit = cmd{
		cmd:      "quit",
		required: false,
		n:        0,
	}
	CommandRename = cmd{
		cmd:      "mv",
		required: true,
		n:        2,
	}
	CommandGet = cmd{
		cmd:      "get",
		required: true,
		n:        -1,
	}
	CommandPut = cmd{
		cmd:      "put",
		required: true,
		n:        -1,
	}

	commandsTable map[string]cmd
)

func init() {
	commandsTable = make(map[string]cmd)
	commandsTable[CommandAuthTLS.cmd] = CommandAuthTLS
	commandsTable[CommandAuthSSL.cmd] = CommandAuthSSL
	commandsTable[CommandAbort.cmd] = CommandAbort
	commandsTable[CommandCd.cmd] = CommandCd
	commandsTable[CommandRm.cmd] = CommandRm
	commandsTable[CommandFileInfo.cmd] = CommandFileInfo
	commandsTable[CommandLs.cmd] = CommandLs
	commandsTable[CommandGet.cmd] = CommandGet
	commandsTable[CommandPut.cmd] = CommandPut
	commandsTable[CommandPwd.cmd] = CommandPwd
	commandsTable[CommandQuit.cmd] = CommandQuit
	commandsTable[CommandMkdir.cmd] = CommandMkdir
	commandsTable[CommandNoop.cmd] = CommandNoop
	commandsTable[CommandRename.cmd] = CommandRename
}
