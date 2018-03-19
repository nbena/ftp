// Copyright 2018 nbena
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
