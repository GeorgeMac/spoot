Spoot - Long Running Shells
---------------------------

Control long running shells in other processes using only Stdin and Stdout.

> New Dog, Old Trick: https://github.com/mitchellh/net-ssh-shell/blob/master/lib/net/ssh/shell/process.rb#L46

This is a Go re-interpretation of a trick done by a ruby library called `net-ssh-shell`. While it is probably not a good idea in general, it is still relatively effective and gets the job done.

```go
bash := exec.Command("bash")

shell, err := spoot.NewShell(bash)
if err != nil {
	bash.Process.Kill()
	panic(err)
}

bash.Start()

cmd := spoot.NewCommand("echo 'foo'")
cmd.Stdout = os.Stdout
exitCode, err := shell.Exec(cmd)
if err != nil {
	bash.Process.Kill()
	panic(err)
}

fmt.Printf("Command finished with exit code %d\n", exitCode)

cmd = spoot.NewCommand("echo 'bar' && exit 1")
cmd.Stdout = os.Stdout
exitCode, err = shell.Exec(cmd)
if err != nil {
	bash.Process.Kill()
	panic(err)
}

fmt.Printf("Command finished with exit code %d\n", exitCode)

bash.Process.Kill()
```

## Roadmap

- [ ] tests
- [ ] examples with `os/exec`, `net/ssh` and `docker`
- [ ] finish stderr support
- [ ] improve close handling

## Caveats

- You can only execute one command at a time per long running shell. No concurrent use of `shell.Exec`.
