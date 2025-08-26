package ssh

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/QingMing-bot/ipmi-ssh-tool/config"
	"golang.org/x/crypto/ssh"
)

// TestAuth 测试SSH免密登录
func TestAuth(m config.Machine) error {
	privateKeyPath := config.GetLocalSSHKeyPath()[:len(config.GetLocalSSHKeyPath())-4]
	keyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return err
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return err
	}

	sshConfig := &ssh.ClientConfig{
		User:            m.SSHUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		Timeout:         10 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", m.SSHIP+":22", sshConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	return nil
}

// BatchOperate 批量SSH操作
func BatchOperate(ms config.Machines, operateType string, cmd string) []string {
	results := make([]string, len(ms))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // 限制并发数

	for i, m := range ms {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, machine config.Machine) {
			defer func() {
				<-sem
				wg.Done()
			}()

			prefix := fmt.Sprintf("[%d/%d] %s", idx+1, len(ms), machine.SSHIP)
			var result string

			switch operateType {
			case "connect":
				if err := openSSHInteractive(machine); err != nil {
					result = fmt.Sprintf("%s: 连接失败 - %s", prefix, err)
				} else {
					result = fmt.Sprintf("%s: 连接已建立", prefix)
				}
			case "command":
				if output, err := execSSHCommand(machine, cmd); err != nil {
					result = fmt.Sprintf("%s: 命令失败 - %s\n输出: %s", prefix, err, output)
				} else {
					result = fmt.Sprintf("%s: 命令成功\n输出: %s", prefix, output)
				}
			default:
				result = fmt.Sprintf("%s: 未知操作类型", prefix)
			}

			results[idx] = result
		}(i, m)
	}

	wg.Wait()
	return results
}

// openSSHInteractive 打开SSH交互终端
func openSSHInteractive(m config.Machine) error {
	sshAddr := fmt.Sprintf("%s@%s", m.SSHUser, m.SSHIP)

	switch runtime.GOOS {
	case "windows":
		return exec.Command("wt", "ssh", sshAddr).Start()
	case "darwin":
		return exec.Command("open", "-a", "Terminal", "ssh", sshAddr).Start()
	default:
		return exec.Command("x-terminal-emulator", "-e", "ssh", sshAddr).Start()
	}
}

// execSSHCommand 执行SSH命令
func execSSHCommand(m config.Machine, cmdStr string) (string, error) {
	privateKeyPath := config.GetLocalSSHKeyPath()[:len(config.GetLocalSSHKeyPath())-4]
	keyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return "", err
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return "", err
	}

	sshConfig := &ssh.ClientConfig{
		User:            m.SSHUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		Timeout:         10 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", m.SSHIP+":22", sshConfig)
	if err != nil {
		return "", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(cmdStr); err != nil {
		return stderr.String(), err
	}
	return stdout.String(), nil
}
