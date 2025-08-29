package domain

import "time"

// Machine 统一的机器领域模型
// 注意: remark / created_at 在部分早期表结构可能不存在；请保证迁移后包含
type Machine struct {
	ID        int       `json:"id"`
	IPMIIP    string    `json:"ipmi_ip"`  // IPMI管理IP
	SSHIP     string    `json:"ssh_ip"`   // SSH连接IP
	SSHUser   string    `json:"ssh_user"` // SSH用户名
	SSHKey    string    `json:"-"`        // 私钥（不序列化）
	Remark    string    `json:"remark,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}
