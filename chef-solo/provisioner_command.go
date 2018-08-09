package chef_solo

import (
	"context"
	"fmt"
	"github.com/armon/circbuf"
	"github.com/hashicorp/terraform/communicator"
	"github.com/hashicorp/terraform/communicator/remote"
	"github.com/hashicorp/terraform/terraform"
	"github.com/mitchellh/go-linereader"
	"io"
	"os"
	"os/exec"
	"runtime"
)

func (p *provisioner) runLocal(ctx context.Context, o terraform.UIOutput, command string) error {

	if command == "" {
		return fmt.Errorf("local-exec provisioner command must be a non-empty string")
	}

	// Execute the command with env
	environment := make(map[string]interface{})

	var env []string
	env = make([]string, len(environment))
	for k := range environment {
		entry := fmt.Sprintf("%s=%s", k, environment[k].(string))
		env = append(env, entry)
	}

	// Execute the command using a shell
	interpreter := make([]interface{}, 0)

	var cmdargs []string
	if len(interpreter) > 0 {
		for _, i := range interpreter {
			if arg, ok := i.(string); ok {
				cmdargs = append(cmdargs, arg)
			}
		}
	} else {
		if runtime.GOOS == "windows" {
			cmdargs = []string{"cmd", "/C"}
		} else {
			cmdargs = []string{"/bin/sh", "-c"}
		}
	}
	cmdargs = append(cmdargs, command)

	// Setup the reader that will read the output from the command.
	// We use an os.Pipe so that the *os.File can be passed directly to the
	// process, and not rely on goroutines copying the data which may block.
	// See golang.org/issue/18874
	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to initialize pipe for output: %s", err)
	}

	var cmdEnv []string
	cmdEnv = os.Environ()
	cmdEnv = append(cmdEnv, env...)

	// Setup the command
	cmd := exec.CommandContext(ctx, cmdargs[0], cmdargs[1:]...)
	cmd.Stderr = pw
	cmd.Stdout = pw
	// Dir specifies the working directory of the command.
	// If Dir is the empty string (this is default), runs the command
	// in the calling process's current directory.
	cmd.Dir = ""
	// Env specifies the environment of the command.
	// By default will use the calling process's environment
	cmd.Env = cmdEnv

	output, _ := circbuf.NewBuffer(maxBufSize)

	// Write everything we read from the pipe to the output buffer too
	tee := io.TeeReader(pr, output)

	// copy the teed output to the UI output
	copyDoneCh := make(chan struct{})
	go copyOutputLocal(o, tee, copyDoneCh)

	// Output what we're about to run
	o.Output(fmt.Sprintf("Executing: %q", cmdargs))

	// Start the command
	err = cmd.Start()
	if err == nil {
		err = cmd.Wait()
	}

	// Close the write-end of the pipe so that the goroutine mirroring output
	// ends properly.
	pw.Close()

	// Cancelling the command may block the pipe reader if the file descriptor
	// was passed to a child process which hasn't closed it. In this case the
	// copyOutput goroutine will just hang out until exit.
	select {
	case <-copyDoneCh:
	case <-ctx.Done():
	}

	if err != nil {
		return fmt.Errorf("error running command '%s': %v. Output: %s",
			command, err, output.Bytes())
	}

	return nil
}

// runRemote is used to run already prepared commands
func (p *provisioner) runRemote(o terraform.UIOutput, comm communicator.Communicator, command string) error {
	// Unless prevented, prefix the command with sudo
	if p.useSudo {
		command = "sudo bash -c '" + command + "'"
	}

	outR, outW := io.Pipe()
	errR, errW := io.Pipe()
	go copyOutputRemote(o, outR)
	go copyOutputRemote(o, errR)
	defer outW.Close()
	defer errW.Close()

	cmd := &remote.Cmd{
		Command: command,
		Stdout:  outW,
		Stderr:  errW,
	}

	err := comm.Start(cmd)
	if err != nil {
		return fmt.Errorf("error executing command %q: %v", cmd.Command, err)
	}

	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func (p *provisioner) runMultipleCommands(o terraform.UIOutput, comm communicator.Communicator, commands []string) error {
	for _, command := range commands {
		if err := p.runRemote(o, comm, command); err != nil {
			return err
		}
	}
	return nil
}

func copyOutputRemote(o terraform.UIOutput, r io.Reader) {
	lr := linereader.New(r)
	for line := range lr.Ch {
		o.Output(line)
	}
}

func copyOutputLocal(o terraform.UIOutput, r io.Reader, doneCh chan<- struct{}) {
	defer close(doneCh)
	lr := linereader.New(r)
	for line := range lr.Ch {
		o.Output(line)
	}
}
