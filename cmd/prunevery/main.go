// chris 090615

// prunevery enforces a minimum period between executions of a command.
//
//	usage: prunevery period command [argument ...]
//
// period is a non-negative time.Duration.  If period is zero, no
// minimum period will be enforced.
//
// Stat File
//
// prunevery enforces the minimum period execution interval by means of
// examining and updating the modification time on a stat file.  The
// stat file is stored in the default temporary directory (via
// os.TempDir).
//
// The stat file name is generated by producing a a deterministic and
// reasonably human-readable string that identifies the command being
// run.  All non-word characters are consolidated between the command
// and its arguments and are replaced with underscores.
//
// If the length of this string exceeds a reasonable limit, then it will
// be truncated and the suffix will be a hash of the full string so as
// to stay within the length limit, but still uniquely and
// deterministically identify the given command and its arguments.
//
// Diagnostics
//
// prunevery may return with the following exit codes.
//
//	  1 An unidentified error occurred when trying to run or wait on
//	    the command.
//	  2 Invalid arguments.
//	 40 Minimum period not yet elapsed.
//	 41 Error opening, creating, examining, or updating the stat
//	    file.
//	127 The command could not be found.
//
// Except in the case of the minimum period having not yet elapsed, it
// will print an appropriate message to standard error.
//
// In addition, prunevery may return with the following exit code.
//
//	255 The command exited unsuccessfully, but the underlying
//	    operating system does not support examining the exit status.
//
// Otherwise, prunevery will return with the exit code of the command.
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"path/filepath"

	"chrispennello.com/go/prun/cmd"
)

var state struct {
	cmd cmd.State

	// Name of stat file to track last execution.
	statname string

	// Minimum periodic execution interval to enforce.
	period time.Duration
}

func init() {
	log.SetFlags(0)
	state.cmd = cmd.Parse("period")

	var err error
	state.period, err = time.ParseDuration(state.cmd.Me.Args[0])
	if err != nil {
		cmd.ArgError(err)
	}
	if state.period < 0 {
		cmd.BadArgs("period must be non-negative")
	}

	tmp := os.TempDir()
	key := cmd.MakeKey(state.cmd.Cmd.Name, state.cmd.Cmd.Args)
	state.statname = filepath.Join(tmp, fmt.Sprintf("%s_%s", state.cmd.Me.Name, key))
}

// chillopen opens the file without erroring if it already exists.
func chillopen() *os.File {
	flag := os.O_RDONLY | os.O_CREATE
	file, err := os.OpenFile(state.statname, flag, 0666)
	if err != nil {
		log.Print(err)
		os.Exit(41)
	}
	return file
}

func shouldrun() bool {
	flag := os.O_RDONLY | os.O_CREATE | os.O_EXCL
	file, err := os.OpenFile(state.statname, flag, 0666)
	if err == nil {
		file.Close()
		return true
	}
	if !os.IsExist(err) {
		log.Print(err)
		os.Exit(41)
	}
	// os.IsExist(err) == true
	file = chillopen()
	defer file.Close()
	fi, err2 := file.Stat()
	if err2 != nil {
		log.Print(err2)
		os.Exit(41)
	}
	now := time.Now()
	if now.After(fi.ModTime().Add(state.period)) {
		if err := os.Chtimes(state.statname, now, now); err != nil {
			log.Print(err)
			os.Exit(41)
		}
		return true
	}
	return false
}

func main() {
	if state.period > 0 && !shouldrun() {
		os.Exit(40)
	}
	proc := cmd.NewProc(state.cmd.Cmd.Name, state.cmd.Cmd.Args)
	proc.Cmd.Stdout = os.Stdout
	proc.Cmd.Stderr = os.Stderr
	proc.StartExit()
	proc.WaitExit()
}