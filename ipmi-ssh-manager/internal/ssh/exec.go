package ssh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	gssh "golang.org/x/crypto/ssh"
)

// Executor 是一个简单的 SSH 执行器，实现 Exec(ctx, user, addr, key, cmd, timeout)
// 按需创建/复用连接；可设置最大并发。
type Executor struct {
	pool *ConnectionPool
	sem  chan struct{}
}

// NewExecutor 创建执行器。maxParallel <=0 表示不限制。
func NewExecutor(maxParallel int) *Executor {
	var sem chan struct{}
	if maxParallel > 0 {
		sem = make(chan struct{}, maxParallel)
	}
	return &Executor{pool: NewConnectionPool(), sem: sem}
}

// Exec 执行命令并返回 stdout/stderr/exitCode。
func (e *Executor) Exec(ctx context.Context, user, addr, authMode, keyOrPass, cmd string, timeout time.Duration) (string, string, int, error) {
	if user == "" || addr == "" {
		return "", "", -1, errors.New("user/addr empty")
	}
	if cmd == "" {
		return "", "", -1, errors.New("cmd empty")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if e.sem != nil {
		e.sem <- struct{}{}
		defer func() { <-e.sem }()
	}

	// 获取/建立连接
	client, err := e.pool.Get(user, addr, authMode+":"+keyOrPass)
	if err != nil {
		return "", "", -1, err
	}

	// session
	session, err := client.NewSession()
	if err != nil {
		return "", "", -1, err
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	done := make(chan error, 1)
	go func() { done <- session.Run(cmd) }()

	select {
	case <-ctx.Done():
		// 强制关闭底层连接以中断
		_ = client.Close()
		return stdout.String(), stderr.String(), -1, context.DeadlineExceeded
	case err = <-done:
	}

	exitCode := 0
	if err != nil {
		if ee, ok := err.(*gssh.ExitError); ok {
			exitCode = ee.ExitStatus()
		} else {
			return stdout.String(), stderr.String(), -1, err
		}
	}
	return stdout.String(), stderr.String(), exitCode, nil
}

// StreamExec 以流式方式执行命令，实时回调标准输出/错误。回调参数 isErr 表示是否来自 stderr。
// 最终返回完整 stdout/stderr 与 exitCode。
func (e *Executor) StreamExec(ctx context.Context, user, addr, authMode, keyOrPass, cmd string, timeout time.Duration, onChunk func(data []byte, isErr bool)) (string, string, int, error) {
	if user == "" || addr == "" {
		return "", "", -1, errors.New("user/addr empty")
	}
	if cmd == "" {
		return "", "", -1, errors.New("cmd empty")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if e.sem != nil {
		e.sem <- struct{}{}
		defer func() { <-e.sem }()
	}
	client, err := e.pool.Get(user, addr, authMode+":"+keyOrPass)
	if err != nil {
		return "", "", -1, err
	}
	session, err := client.NewSession()
	if err != nil {
		return "", "", -1, err
	}
	defer session.Close()
	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return "", "", -1, err
	}
	stderrPipe, err := session.StderrPipe()
	if err != nil {
		return "", "", -1, err
	}
	if err = session.Start(cmd); err != nil {
		return "", "", -1, err
	}
	// 读取循环
	var stdoutBuf, stderrBuf bytes.Buffer
	wg := sync.WaitGroup{}
	// 使用 io.ReadCloser (stdoutPipe/stderrPipe) 包装到 goroutine 中
	wg.Add(2)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, er := stdoutPipe.Read(buf)
			if n > 0 {
				b := buf[:n]
				stdoutBuf.Write(b)
				if onChunk != nil {
					onChunk(b, false)
				}
			}
			if er != nil {
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, er := stderrPipe.Read(buf)
			if n > 0 {
				b := buf[:n]
				stderrBuf.Write(b)
				if onChunk != nil {
					onChunk(b, true)
				}
			}
			if er != nil {
				return
			}
		}
	}()
	// 等待结束
	waitCh := make(chan error, 1)
	go func() { waitCh <- session.Wait() }()
	var runErr error
	select {
	case <-ctx.Done():
		_ = client.Close()
		runErr = context.DeadlineExceeded
	case runErr = <-waitCh:
	}
	wg.Wait()
	exitCode := 0
	if runErr != nil {
		if ee, ok := runErr.(*gssh.ExitError); ok {
			exitCode = ee.ExitStatus()
		} else {
			return stdoutBuf.String(), stderrBuf.String(), -1, runErr
		}
	}
	return stdoutBuf.String(), stderrBuf.String(), exitCode, nil
}

// -------- 连接池实现 (简化版) --------

type poolKey string

func makeKey(user, addr, key string) poolKey {
	h := sha256.Sum256([]byte(user + "@" + addr + "|" + key))
	return poolKey(hex.EncodeToString(h[:8]))
}

type ConnectionPool struct {
	mu      sync.Mutex
	clients map[poolKey]*gssh.Client
}

func NewConnectionPool() *ConnectionPool { return &ConnectionPool{clients: map[poolKey]*gssh.Client{}} }

func (p *ConnectionPool) Get(user, addr, key string) (*gssh.Client, error) {
	pk := makeKey(user, addr, key)
	p.mu.Lock()
	if c, ok := p.clients[pk]; ok {
		p.mu.Unlock()
		// 简单健康检测 (非阻塞)
		_, _, err := c.SendRequest("keepalive@openssh.com", true, nil)
		if err == nil {
			return c, nil
		}
		// 失效，移除并重新创建
		p.mu.Lock()
		delete(p.clients, pk)
		p.mu.Unlock()
	} else {
		p.mu.Unlock()
	}

	// key 现在包含 authMode:secret 前缀，用于区分密码/密钥
	var authMethods []gssh.AuthMethod
	if strings.HasPrefix(key, "key:") {
		priv := strings.TrimPrefix(key, "key:")
		signer, err := gssh.ParsePrivateKey([]byte(priv))
		if err != nil {
			return nil, fmt.Errorf("parse key: %w", err)
		}
		authMethods = []gssh.AuthMethod{gssh.PublicKeys(signer)}
	} else if strings.HasPrefix(key, "password:") {
		pass := strings.TrimPrefix(key, "password:")
		authMethods = []gssh.AuthMethod{gssh.Password(pass)}
	} else {
		return nil, fmt.Errorf("unknown auth prefix")
	}
	conf := &gssh.ClientConfig{User: user, Auth: authMethods, HostKeyCallback: gssh.InsecureIgnoreHostKey(), Timeout: 10 * time.Second}
	// 支持 host:port 或仅 host
	target := addr
	if _, _, errSplit := net.SplitHostPort(addr); errSplit != nil {
		target = addr + ":22"
	}
	c, err := gssh.Dial("tcp", target, conf)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.clients[pk] = c
	p.mu.Unlock()
	return c, nil
}

func (p *ConnectionPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for k, c := range p.clients {
		_ = c.Close()
		delete(p.clients, k)
	}
}
