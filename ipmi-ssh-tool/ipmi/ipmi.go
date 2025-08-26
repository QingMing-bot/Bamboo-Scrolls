package ipmi

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/QingMing-bot/ipmi-ssh-tool/config"
)

// ConfigSSH 通过IPMITool远程配置目标机器的SSH免密
// ipmi/config.go
func ConfigSSH(m config.Machine, localPubKey string) error {
	remoteCmd := fmt.Sprintf(
		"mkdir -p /home/%s/.ssh && "+
			"echo '%s' >> /home/%s/.ssh/authorized_keys && "+
			"chown -R %s:%s /home/%s/.ssh && "+
			"chmod 700 /home/%s/.ssh && "+
			"chmod 600 /home/%s/.ssh/authorized_keys",
		m.SSHUser, localPubKey, m.SSHUser,
		m.SSHUser, m.SSHUser, m.SSHUser,
		m.SSHUser, m.SSHUser,
	)

	// 使用原始命令通道（避免SOL激活）
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command(
			"ipmitool",
			"-I", "lanplus",
			"-H", m.IPMIIP,
			"-U", m.IPMIUser,
			"-P", m.IPMIPwd,
			"raw", "0x34", "0x76", // 使用原始命令通道
			strconv.Quote(remoteCmd), // Windows需要额外引号
		)
	} else {
		cmd = exec.Command(
			"ipmitool",
			"-I", "lanplus",
			"-H", m.IPMIIP,
			"-U", m.IPMIUser,
			"-P", m.IPMIPwd,
			"exec", remoteCmd,
		)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("IPMI配置失败: %w\n输出: %s", err, string(output))
	}
	return nil
}

// ConfigSSHForAll 为所有机器配置SSH免密
func ConfigSSHForAll(ms config.Machines) error {
	pubKey, err := config.GetLocalSSHKey()
	if err != nil {
		return err
	}

	for _, m := range ms {
		if err := ConfigSSH(m, pubKey); err != nil {
			return err
		}
	}
	return nil
}
