package importexport

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"strings"

	"github.com/QingMing-Bot/ipmi-ssh-manager/internal/domain"
)

// ParseMachinesJSON 解析 JSON 数组为机器列表
func ParseMachinesJSON(data []byte) ([]domain.Machine, error) {
	var ms []domain.Machine
	if err := json.Unmarshal(data, &ms); err != nil {
		return nil, err
	}
	out := ms[:0]
	for _, m := range ms {
		if strings.TrimSpace(m.IPMIIP) != "" {
			out = append(out, m)
		}
	}
	return out, nil
}

// ParseMachinesCSV 解析 CSV (含 header) -> machines
func ParseMachinesCSV(data []byte) ([]domain.Machine, error) {
	r := csv.NewReader(strings.NewReader(string(data)))
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []domain.Machine{}, nil
	}
	start := 0
	if len(rows[0]) > 0 && strings.Contains(strings.ToLower(strings.Join(rows[0], ",")), "ipmi") {
		start = 1
	}
	var out []domain.Machine
	for i := start; i < len(rows); i++ {
		cols := rows[i]
		if len(cols) == 0 {
			continue
		}
		ipmi := strings.TrimSpace(cols[0])
		if ipmi == "" {
			continue
		}
		m := domain.Machine{IPMIIP: ipmi}
		if len(cols) > 1 {
			m.SSHIP = strings.TrimSpace(cols[1])
		}
		if len(cols) > 2 {
			m.SSHUser = strings.TrimSpace(cols[2])
		}
		if len(cols) > 3 {
			m.SSHKey = strings.TrimSpace(cols[3])
		}
		if len(cols) > 4 {
			m.Remark = strings.TrimSpace(cols[4])
		}
		out = append(out, m)
	}
	return out, nil
}

// RenderMachinesCSV 输出 CSV 字符串 (含 header)
func RenderMachinesCSV(ms []domain.Machine) string {
	var b strings.Builder
	b.WriteString("ipmi_ip,ssh_ip,ssh_user,ssh_key,remark\n")
	for _, m := range ms {
		b.WriteString(strings.Join([]string{
			escapeCSV(m.IPMIIP), escapeCSV(m.SSHIP), escapeCSV(m.SSHUser), escapeCSV(m.SSHKey), escapeCSV(m.Remark),
		}, ","))
		b.WriteString("\n")
	}
	return b.String()
}

func escapeCSV(s string) string {
	if strings.ContainsAny(s, ",\n\"") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

// SerializeMachinesJSON 输出 JSON 字符串
func SerializeMachinesJSON(ms []domain.Machine) (string, error) {
	b, err := json.Marshal(ms)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Simple validation
func ValidateMachines(ms []domain.Machine) error {
	for _, m := range ms {
		if strings.TrimSpace(m.IPMIIP) == "" {
			return errors.New("empty ipmi_ip")
		}
	}
	return nil
}
