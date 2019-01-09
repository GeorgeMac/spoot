package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofrs/uuid"
)

var (
	defaultBufSize = 4096

	matcher = regexp.MustCompilePOSIX(`^([a-zA-Z0-9-]{36}) ([0-9]+)$`)

	_ Piper = (*exec.Cmd)(nil)
)

func main() {
	bash := exec.Command("bash")

	shell, err := NewShell(bash)
	if err != nil {
		bash.Process.Kill()
		panic(err)
	}

	bash.Start()

	cmd := NewCommand("echo 'foo'")
	cmd.Stdout = os.Stdout
	exitCode, err := shell.Exec(cmd)
	if err != nil {
		bash.Process.Kill()
		panic(err)
	}

	fmt.Printf("Command finished with exit code %d\n", exitCode)

	cmd = NewCommand("echo 'bar' && exit 1")
	cmd.Stdout = os.Stdout
	exitCode, err = shell.Exec(cmd)
	if err != nil {
		bash.Process.Kill()
		panic(err)
	}

	fmt.Printf("Command finished with exit code %d\n", exitCode)

	bash.Process.Kill()
}

type Shell struct {
	io.WriteCloser

	stdout io.ReadCloser
	stderr io.ReadCloser
}

func NewShell(piper Piper) (*Shell, error) {
	stdin, err := piper.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := piper.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := piper.StderrPipe()
	if err != nil {
		return nil, err
	}

	return &Shell{WriteCloser: stdin, stdout: stdout, stderr: stderr}, nil
}

type Piper interface {
	StdinPipe() (io.WriteCloser, error)
	OutPiper
}

type OutPiper interface {
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
}

type Command struct {
	Args   []string
	Stdout io.Writer
	Stderr io.Writer
}

func NewCommand(cmd string) Command {
	return Command{Args: []string{cmd}}
}

func (shell *Shell) Exec(cmd Command) (int, error) {
	key, err := uuid.NewGen().NewV4()
	if err != nil {
		return -1, err
	}

	intercept := NewInterceptReader(shell.stdout, key)

	done := make(chan struct{})
	go func() {
		defer close(done)
		cmd.feedStdout(context.TODO(), intercept)
		// cmd.feedStderr(context.TODO(), shell.stderr)
	}()

	cmdStr := fmt.Sprintf(`sh -c %q;`, strings.Join(cmd.Args, " "))

	fmt.Fprintf(shell, "%s DONTEVERUSETHIS=$?; echo %s $DONTEVERUSETHIS; echo \"exit $DONTEVERUSETHIS\"|sh\n", cmdStr, key.String())

	<-done

	return int(intercept.ExitCode), nil
}

func (c Command) feedStdout(ctxt context.Context, r io.Reader) (int64, error) {
	return safeCopy(ctxt, c.Stdout, r)
}

func (c Command) feedStderr(ctxt context.Context, r io.Reader) (int64, error) {
	return safeCopy(ctxt, c.Stderr, r)
}

func safeCopy(ctxt context.Context, dst io.Writer, src io.Reader) (int64, error) {
	if dst == nil {
		dst = ioutil.Discard
	}

	return io.Copy(dst, src)
}

type InterceptReader struct {
	Key      uuid.UUID
	ExitCode int64

	r *bufio.Reader

	buf  []byte
	read int
}

func NewInterceptReader(r io.Reader, key uuid.UUID) *InterceptReader {
	return &InterceptReader{r: bufio.NewReaderSize(r, defaultBufSize), Key: key, ExitCode: -1}
}

func (r *InterceptReader) Read(p []byte) (n int, err error) {
	if r.ExitCode >= 0 {
		return 0, io.EOF
	}

	if r.read == len(r.buf) {
		r.buf, err = r.r.ReadSlice('\n')
		if err != nil || err == io.EOF {
			return
		}

		r.read = 0

		matches := matcher.FindAllSubmatch(r.buf, 1)
		if len(matches) > 0 && len(matches[0]) == 3 {
			key, uerr := uuid.FromString(string(matches[0][1]))
			// if it is a uuid and the key matches
			if uerr == nil && key == r.Key {
				// we've reached the end of intercepted command
				r.ExitCode, err = strconv.ParseInt(string(matches[0][2]), 10, 32)
				if err != nil {
					// something very wrong as regexp matched numbers which should be parseable
					panic(err)
				}

				err = io.EOF
				return
			}
		}
	}

	n = copy(p, r.buf[r.read:])

	r.read += n

	return
}
