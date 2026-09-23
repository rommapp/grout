//go:build e2e

package e2e

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// transport carries a command or a file to a handheld.
//
// ADB and SSH differ only in how they are spelled, so the rest of the device
// target is written against this and does not care which cable is in use.
type transport interface {
	// describe names the connection, for a test's output.
	describe() string

	// run executes a shell command on the device and returns what it printed.
	run(timeout time.Duration, command string) ([]byte, error)

	// start begins a command that outlives the call, so the caller can kill
	// it later.
	start(command string) (*exec.Cmd, error)

	// push copies a file from this machine to the device.
	push(local, remote string) error

	// pull copies a file from the device to this machine.
	pull(remote, local string) error

	// available reports whether a device is actually on the other end.
	available() error
}

// openTransport reads a spec and returns the connection it names.
//
//	adb              the only device adb can see
//	adb:<serial>     that device, when several are plugged in
//	ssh:<user@host>  anything with a shell, including a device on wifi
func openTransport(spec string) (transport, error) {
	kind, rest, _ := strings.Cut(spec, ":")

	switch kind {
	case "adb":
		return &adbTransport{serial: rest, scratch: fmt.Sprintf("/tmp/.grout-e2e-%d", os.Getpid())}, nil
	case "ssh":
		if rest == "" {
			return nil, fmt.Errorf("ssh needs a destination, as ssh:root@10.0.0.5")
		}
		return &sshTransport{dest: rest}, nil
	default:
		return nil, fmt.Errorf("unknown device %q, want adb, adb:<serial> or ssh:<user@host>", spec)
	}
}

// adbTransport talks to a device over ADB.
//
// The handhelds run a cut down adbd: `adb shell` is all it offers, and that
// one command both swallows the exit status and rewrites every newline. So
// each command is wrapped to carry its own status and its output comes back
// base64 encoded, which survives the rewriting.
//
// `adb exec-out`, which has neither problem, is not used: none of the devices
// this is for implement it.
type adbTransport struct {
	serial string
	// scratch is where a wrapped command parks its output, unique per
	// connection so two runs cannot read each other's.
	scratch string
}

func (a *adbTransport) describe() string {
	if a.serial == "" {
		return "the attached adb device"
	}
	return "adb device " + a.serial
}

// prefix is the serial selector, when one was asked for.
func (a *adbTransport) prefix() []string {
	if a.serial == "" {
		return nil
	}
	return []string{"-s", a.serial}
}

// the markers a wrapped command prints around each section of its answer.
const (
	rcMarker  = "__rc="
	errMarker = "__stderr"
)

// wrap rewrites a command so its status and its exact output survive the trip.
//
// The status is printed before the output rather than after, so the order is
// the shell's and not a race between two buffers.
func (a *adbTransport) wrap(command string) string {
	out, errs := a.scratch+".out", a.scratch+".err"
	// A subshell rather than a group, or a command that calls exit takes the
	// whole wrapper down with it and nothing is reported at all.
	return fmt.Sprintf("( %s ) > %s 2> %s; echo \"%s$?\"; base64 < %s; echo %s; base64 < %s; rm -f %s %s",
		command, out, errs, rcMarker, out, errMarker, errs, out, errs)
}

func (a *adbTransport) run(timeout time.Duration, command string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := append(a.prefix(), "shell", a.wrap(command))
	raw, err := exec.CommandContext(ctx, "adb", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("adb shell: %w", err)
	}
	return unwrap(raw, command)
}

// unwrap reads back what a wrapped command answered.
func unwrap(raw []byte, command string) ([]byte, error) {
	// The newline rewriting is exactly what base64 is here to survive, but the
	// markers go through it too.
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")

	_, rest, found := strings.Cut(text, rcMarker)
	if !found {
		return nil, fmt.Errorf("the device did not answer %q, it said %q", command, trim(text))
	}

	status, rest, _ := strings.Cut(rest, "\n")
	encoded, encodedErr, _ := strings.Cut(rest, errMarker)

	out, decodeErr := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(encoded), ""))
	if decodeErr != nil {
		return nil, fmt.Errorf("the device garbled its answer to %q: %w", command, decodeErr)
	}

	if strings.TrimSpace(status) != "0" {
		complaint, _ := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(encodedErr), ""))
		return out, fmt.Errorf("%q failed on the device with status %s: %s",
			command, strings.TrimSpace(status), trim(string(complaint)))
	}
	return out, nil
}

// trim keeps a device's complaint short enough to read in a failure.
func trim(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 400 {
		return s[:400] + "..."
	}
	return s
}

// start runs a command unwrapped, because its output is not read back and it
// has to keep running after this returns.
func (a *adbTransport) start(command string) (*exec.Cmd, error) {
	cmd := exec.Command("adb", append(a.prefix(), "shell", command)...)
	return cmd, cmd.Start()
}

func (a *adbTransport) push(local, remote string) error {
	_, err := output(exec.Command("adb", append(a.prefix(), "push", local, remote)...))
	return err
}

func (a *adbTransport) pull(remote, local string) error {
	_, err := output(exec.Command("adb", append(a.prefix(), "pull", remote, local)...))
	return err
}

// available asks adb what it can see, and insists on exactly one answer unless
// a serial picked one out.
func (a *adbTransport) available() error {
	out, err := output(exec.Command("adb", "devices"))
	if err != nil {
		return fmt.Errorf("adb is not usable: %w", err)
	}

	var attached []string
	for _, line := range strings.Split(string(out), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == "device" {
			attached = append(attached, fields[0])
		}
	}

	switch {
	case len(attached) == 0:
		return fmt.Errorf("adb sees no device")
	case a.serial != "":
		for _, serial := range attached {
			if serial == a.serial {
				return nil
			}
		}
		return fmt.Errorf("adb cannot see %s, only %s", a.serial, strings.Join(attached, ", "))
	case len(attached) > 1:
		return fmt.Errorf("adb sees several devices (%s), name one with adb:<serial>", strings.Join(attached, ", "))
	}
	return nil
}

// sshTransport talks to anything with a shell.
type sshTransport struct{ dest string }

func (s *sshTransport) describe() string { return "ssh to " + s.dest }

// sshArgs keeps a run from stopping on a prompt: a handheld's key changes
// whenever its firmware is reflashed, and none of this is a secret channel.
func (s *sshTransport) sshArgs(rest ...string) []string {
	return append([]string{
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
	}, rest...)
}

func (s *sshTransport) run(timeout time.Duration, command string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return output(exec.CommandContext(ctx, "ssh", s.sshArgs(s.dest, command)...))
}

func (s *sshTransport) start(command string) (*exec.Cmd, error) {
	cmd := exec.Command("ssh", s.sshArgs(s.dest, command)...)
	return cmd, cmd.Start()
}

func (s *sshTransport) push(local, remote string) error {
	_, err := output(exec.Command("scp", s.sshArgs(local, s.dest+":"+remote)...))
	return err
}

func (s *sshTransport) pull(remote, local string) error {
	_, err := output(exec.Command("scp", s.sshArgs(s.dest+":"+remote, local)...))
	return err
}

func (s *sshTransport) available() error {
	if _, err := s.run(10*time.Second, "true"); err != nil {
		return fmt.Errorf("cannot reach %s: %w", s.dest, err)
	}
	return nil
}

// output runs a command and folds whatever it complained about into the error,
// which is otherwise just an exit status.
func output(cmd *exec.Cmd) ([]byte, error) {
	var stderr strings.Builder
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(string(out))
		}
		if detail != "" {
			return out, fmt.Errorf("%s: %w: %s", cmd.Path, err, detail)
		}
		return out, fmt.Errorf("%s: %w", cmd.Path, err)
	}
	return out, nil
}

// shellQuote wraps a value so a device's shell takes it literally. Card paths
// and rom names both contain spaces.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
