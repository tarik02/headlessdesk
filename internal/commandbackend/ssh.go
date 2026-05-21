package commandbackend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type SSHConfig struct {
	Host                  string
	Port                  uint16
	Username              string
	Password              string
	PrivateKeyPath        string
	PrivateKeyPassphrase  string
	KnownHostsPath        string
	InsecureIgnoreHostKey bool
	Timeout               time.Duration
}

type sshClient struct {
	client *ssh.Client
}

func newSSHClient(cfg SSHConfig) (*sshClient, error) {
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.Username = strings.TrimSpace(cfg.Username)
	if cfg.Host == "" {
		return nil, errors.New("command.ssh.host is required")
	}
	if cfg.Username == "" {
		return nil, errors.New("command.ssh.username is required")
	}

	auth, err := sshAuthMethods(cfg)
	if err != nil {
		return nil, err
	}
	hostKeyCallback, err := sshHostKeyCallback(cfg)
	if err != nil {
		return nil, err
	}

	client, err := ssh.Dial("tcp", net.JoinHostPort(cfg.Host, fmt.Sprint(cfg.Port)), &ssh.ClientConfig{
		User:            cfg.Username,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         cfg.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("connect ssh %s:%d: %w", cfg.Host, cfg.Port, err)
	}
	return &sshClient{client: client}, nil
}

func (c *sshClient) Run(ctx context.Context, command string, stdin []byte) ([]byte, string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return nil, "", fmt.Errorf("open ssh session: %w", err)
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	if stdin != nil {
		session.Stdin = bytes.NewReader(stdin)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- session.Run(command)
	}()

	select {
	case err := <-errCh:
		return stdout.Bytes(), stderr.String(), err
	case <-ctx.Done():
		_ = session.Close()
		err := <-errCh
		if err == nil {
			err = ctx.Err()
		}
		return stdout.Bytes(), stderr.String(), err
	}
}

func (c *sshClient) Close() error {
	return c.client.Close()
}

func sshAuthMethods(cfg SSHConfig) ([]ssh.AuthMethod, error) {
	var auth []ssh.AuthMethod
	if cfg.Password != "" {
		auth = append(auth, ssh.Password(cfg.Password))
	}
	if cfg.PrivateKeyPath != "" {
		key, err := os.ReadFile(expandHome(cfg.PrivateKeyPath))
		if err != nil {
			return nil, fmt.Errorf("read ssh private key: %w", err)
		}
		var signer ssh.Signer
		if cfg.PrivateKeyPassphrase == "" {
			signer, err = ssh.ParsePrivateKey(key)
		} else {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(cfg.PrivateKeyPassphrase))
		}
		if err != nil {
			return nil, fmt.Errorf("parse ssh private key: %w", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if len(auth) == 0 {
		return nil, errors.New("command.ssh password or private_key_path is required")
	}
	return auth, nil
}

func sshHostKeyCallback(cfg SSHConfig) (ssh.HostKeyCallback, error) {
	if cfg.InsecureIgnoreHostKey {
		return ssh.InsecureIgnoreHostKey(), nil
	}
	if cfg.KnownHostsPath == "" {
		return nil, errors.New("command.ssh.known_hosts_path is required unless insecure_ignore_host_key is true")
	}
	callback, err := knownhosts.New(expandHome(cfg.KnownHostsPath))
	if err != nil {
		return nil, fmt.Errorf("load ssh known_hosts: %w", err)
	}
	return callback, nil
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func shellQuoteArgv(argv []string) string {
	quoted := make([]string, 0, len(argv))
	for _, arg := range argv {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("@%_+=:,./-", r))
	}) == -1 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
