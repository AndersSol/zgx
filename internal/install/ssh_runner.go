package install

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/AndersSol/zgx/internal/connect"
	"golang.org/x/crypto/ssh"
)

// SSHRunner kjører install-motorens kommandoer over SSH med privatnøkkel-auth.
type SSHRunner struct {
	Target         connect.Target
	HostKey        ssh.HostKeyCallback
	PrivateKeyPath string
}

func (r SSHRunner) Run(ctx context.Context, command, sudoPassword string, timeout time.Duration, retries int) (CommandResult, error) {
	if r.HostKey == nil {
		return CommandResult{}, fmt.Errorf("ssh runner: HostKey callback mangler")
	}
	if retries < 0 {
		retries = 0
	}

	signer, err := r.signer()
	if err != nil {
		return CommandResult{}, err
	}

	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		attemptCtx := ctx
		cancel := func() {}
		if timeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, timeout)
		}

		result, err := r.runOnce(attemptCtx, signer, command, sudoPassword)
		cancel()
		if err == nil {
			return result, nil
		}
		lastErr = err
		if ctxErr := ctx.Err(); ctxErr != nil {
			return result, ctxErr
		}
	}

	return CommandResult{}, lastErr
}

func (r SSHRunner) signer() (ssh.Signer, error) {
	privateKey, err := os.ReadFile(r.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("ssh runner: read private key %q: %w", r.PrivateKeyPath, err)
	}
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("ssh runner: parse private key %q: %w", r.PrivateKeyPath, err)
	}
	return signer, nil
}

func (r SSHRunner) runOnce(ctx context.Context, signer ssh.Signer, command, sudoPassword string) (CommandResult, error) {
	if err := ctx.Err(); err != nil {
		return CommandResult{}, err
	}

	config := &ssh.ClientConfig{
		User:            r.Target.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: r.HostKey,
		Timeout:         timeoutFromContext(ctx),
	}

	client, err := ssh.Dial("tcp", r.Target.Addr(), config)
	if err != nil {
		return CommandResult{}, fmt.Errorf("ssh runner: dial %s@%s: %w", r.Target.User, r.Target.Addr(), err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return CommandResult{}, fmt.Errorf("ssh runner: create session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	stdin, err := session.StdinPipe()
	if err != nil {
		return CommandResult{}, fmt.Errorf("ssh runner: stdin pipe: %w", err)
	}

	if err := session.Start(command); err != nil {
		_ = stdin.Close()
		return CommandResult{}, fmt.Errorf("ssh runner: start command: %w", err)
	}

	if strings.HasPrefix(command, "sudo -S") {
		if _, err := io.WriteString(stdin, sudoPassword+"\n"); err != nil {
			_ = stdin.Close()
			return CommandResult{}, fmt.Errorf("ssh runner: write sudo stdin: %w", err)
		}
	}
	if err := stdin.Close(); err != nil {
		return CommandResult{}, fmt.Errorf("ssh runner: close stdin: %w", err)
	}

	type waitResult struct {
		err error
	}
	done := make(chan waitResult, 1)
	go func() {
		done <- waitResult{err: session.Wait()}
	}()

	select {
	case <-ctx.Done():
		_ = client.Close()
		return CommandResult{
			ExitCode: -1,
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
		}, ctx.Err()
	case res := <-done:
		result := CommandResult{
			ExitCode: 0,
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
		}
		if res.err == nil {
			return result, nil
		}

		var exitErr *ssh.ExitError
		if errors.As(res.err, &exitErr) {
			result.ExitCode = exitErr.ExitStatus()
			return result, nil
		}

		return result, fmt.Errorf("ssh runner: wait command: %w", res.err)
	}
}

func timeoutFromContext(ctx context.Context) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		timeout := time.Until(deadline)
		if timeout <= 0 {
			return time.Nanosecond
		}
		return timeout
	}
	return 15 * time.Second
}
