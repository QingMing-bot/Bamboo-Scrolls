package ui

import (
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/QingMing-bot/ipmi-ssh-tool/config"
	"github.com/QingMing-bot/ipmi-ssh-tool/ipmi"
	"github.com/QingMing-bot/ipmi-ssh-tool/ssh"
)

// MainWindow 创建主窗口
func MainWindow(app fyne.App) fyne.Window {
	win := app.NewWindow("IPMI-SSH 批量管理工具")
	win.Resize(fyne.NewSize(1000, 600))

	// 页面容器
	stack := container.NewStack()

	// 创建3个页面
	pages := []fyne.CanvasObject{
		newInputPage(stack),
		newConfigPage(stack),
		newSSHOperatePage(stack),
	}

	for _, page := range pages {
		stack.Add(page)
	}
	stack.Objects = pages // 确保 Objects 被正确赋值
	stack.ShowOnly(pages[0])

	win.SetContent(stack)
	return win
}

// newInputPage 机器信息录入页
func newInputPage(stack *fyne.Container) fyne.CanvasObject {
	// 初始化数据绑定
	machineData := binding.NewUntypedList()
	if ms, err := config.Load(); err == nil {
		machineData.Set(toInterfaceSlice(ms))
	}

	// 表格组件
	table := widget.NewTable(
		func() (int, int) {
			length := machineData.Length()
			return length, 5
		},
		func() fyne.CanvasObject {
			return widget.NewEntry()
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			entry := cell.(*widget.Entry)
			items, _ := machineData.Get()

			if id.Row >= len(items) {
				return
			}

			m := items[id.Row].(config.Machine)
			switch id.Col {
			case 0:
				entry.SetText(m.IPMIIP)
			case 1:
				entry.SetText(m.IPMIUser)
			case 2:
				entry.SetText(m.IPMIPwd)
				entry.Password = true
			case 3:
				entry.SetText(m.SSHIP)
			case 4:
				entry.SetText(m.SSHUser)
			}

			// 实时更新数据
			entry.OnChanged = func(s string) {
				items, _ := machineData.Get()
				if id.Row >= len(items) {
					return
				}

				m := items[id.Row].(config.Machine)
				switch id.Col {
				case 0:
					m.IPMIIP = s
				case 1:
					m.IPMIUser = s
				case 2:
					m.IPMIPwd = s
				case 3:
					m.SSHIP = s
				case 4:
					m.SSHUser = s
				}

				items[id.Row] = m
				_ = machineData.Set(items)
			}
		},
	)

	table.SetColumnWidth(0, 150)
	table.SetColumnWidth(1, 120)
	table.SetColumnWidth(2, 120)
	table.SetColumnWidth(3, 150)
	table.SetColumnWidth(4, 120)

	// 按钮
	addBtn := widget.NewButton("添加机器", func() {
		items, _ := machineData.Get()
		items = append(items, config.Machine{})
		_ = machineData.Set(items)
		table.Refresh()
	})

	delBtn := widget.NewButton("删除选中", func() {
		selectedRows := table.SelectedRows()
		if selectedRows == nil {
			return
		}
		selected, ok := selectedRows.([]int)
		if !ok || len(selected) == 0 {
			return
		}

		items, _ := machineData.Get()
		newItems := make([]interface{}, 0, len(items)-len(selected))

		for i, item := range items {
			if !contains(selected, i) {
				newItems = append(newItems, item)
			}
		}

		_ = machineData.Set(newItems)
		table.UnselectAll()
		table.Refresh()
	})

	saveBtn := widget.NewButton("保存配置", func() {
		items, _ := machineData.Get()
		var machines config.Machines
		for _, item := range items {
			machines = append(machines, item.(config.Machine))
		}

		if err := machines.Save(); err != nil {
			showDialog("保存失败", err.Error())
		} else {
			showDialog("保存成功", "配置已保存到 machines.json")
		}
	})

	nextBtn := widget.NewButton("下一步", func() {
		count := machineData.Length()
		if count == 0 {
			showDialog("提示", "请先添加机器信息")
			return
		}
		stack.ShowOnly(stack.Objects[1])
	})

	// 布局
	btnBox := container.NewHBox(addBtn, delBtn, saveBtn, nextBtn)
	return container.NewVBox(
		widget.NewLabelWithStyle("1. 机器信息录入", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		table,
		btnBox,
		widget.NewLabel("注：IPMI密码和SSH信息仅本地保存"),
	)
}

// newConfigPage 自动化配置页
func newConfigPage(stack *fyne.Container) fyne.CanvasObject {
	progress := widget.NewProgressBar()
	logText := widget.NewEntry()
	logText.MultiLine = true
	logText.SetReadOnly(true)

	startBtn := widget.NewButton("开始配置", func() {
		logText.SetText("")
		addLog(logText, "开始自动化配置...")

		ms, err := config.Load()
		if err != nil {
			addLog(logText, "加载配置失败: "+err.Error())
			return
		}

		if len(ms) == 0 {
			addLog(logText, "机器配置为空，请返回录入页添加")
			return
		}

		go func() {
			successCount := 0
			for i, m := range ms {
				prefix := fmt.Sprintf("[%d/%d] %s", i+1, len(ms), m.SSHIP)
				addLog(logText, prefix+": 开始配置")

				// 1. 获取公钥
				pubKey, err := config.GetLocalSSHKey()
				if err != nil {
					addLog(logText, prefix+": 获取公钥失败 - "+err.Error())
					continue
				}

				// 2. IPMI配置
				if err := ipmi.ConfigSSH(m, pubKey); err != nil {
					addLog(logText, prefix+": IPMI配置失败 - "+err.Error())
					continue
				}
				addLog(logText, prefix+": IPMI配置成功")

				// 3. SSH测试
				if err := ssh.TestAuth(m); err != nil {
					addLog(logText, prefix+": SSH测试失败 - "+err.Error())
					continue
				}

				addLog(logText, prefix+": 配置完成")
				successCount++
				progress.SetValue(float64(i+1) / float64(len(ms)))
			}

			addLog(logText, fmt.Sprintf("配置完成: 成功%d台，失败%d台",
				successCount, len(ms)-successCount))
			stack.ShowOnly(stack.Objects[2])
		}()
	})

	backBtn := widget.NewButton("返回", func() {
		stack.ShowOnly(stack.Objects[0])
	})

	return container.NewVBox(
		widget.NewLabelWithStyle("2. 自动化配置(IPMI→SSH免密)", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		progress,
		logText,
		container.NewHBox(startBtn, backBtn),
	)
}

// newSSHOperatePage SSH批量操作页
func newSSHOperatePage(stack *fyne.Container) fyne.CanvasObject {
	operateCombo := widget.NewSelect([]string{"批量SSH连接（交互）", "批量执行命令"}, nil)
	operateCombo.SetSelectedIndex(0)

	cmdInput := widget.NewEntry()
	cmdInput.SetPlaceHolder("输入需批量执行的命令（如 df -h、free -m）")
	cmdInput.Hide()

	resultText := widget.NewMultiLineEntry()
	resultText.SetReadOnly(true)

	// 切换操作类型
	operateCombo.OnChanged = func(s string) {
		if s == "批量执行命令" {
			cmdInput.Show()
		} else {
			cmdInput.Hide()
		}
	}

	execBtn := widget.NewButton("执行操作", func() {
		resultText.SetText("")
		addLog(resultText, "开始执行SSH批量操作...")

		ms, err := config.Load()
		if err != nil {
			addLog(resultText, "加载配置失败: "+err.Error())
			return
		}

		if len(ms) == 0 {
			addLog(resultText, "机器配置为空，请先录入")
			return
		}

		operateType := operateCombo.Selected
		cmd := cmdInput.Text

		if operateType == "批量执行命令" {
			if cmd == "" {
				addLog(resultText, "请输入需批量执行的命令")
				return
			}
			addLog(resultText, "批量执行命令: "+cmd)
		}

		go func() {
			var results []string
			if operateType == "批量SSH连接（交互）" {
				results = ssh.BatchOperate(ms, "connect", "")
			} else {
				results = ssh.BatchOperate(ms, "command", cmd)
			}

			for _, res := range results {
				addLog(resultText, res)
			}
			addLog(resultText, "SSH批量操作执行完成")
		}()
	})

	exportBtn := widget.NewButton("导出结果", func() {
		savePath := "ssh_results_" + time.Now().Format("20060102_150405") + ".txt"
		if err := os.WriteFile(savePath, []byte(resultText.Text), 0644); err != nil {
			showDialog("导出失败", err.Error())
		} else {
			showDialog("导出成功", "结果已保存到: "+savePath)
		}
	})

	backBtn := widget.NewButton("返回配置页", func() {
		stack.ShowOnly(stack.Objects[1])
	})

	return container.NewVBox(
		widget.NewLabelWithStyle("3. SSH批量操作", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		operateCombo,
		cmdInput,
		resultText,
		container.NewHBox(execBtn, exportBtn, backBtn),
	)
}

// 辅助函数
func addLog(text *widget.Entry, msg string) {
	now := time.Now().Format("[2006-01-02 15:04:05] ")
	text.SetText(text.Text + now + msg + "\n")
	text.CursorRow = len(text.Text) - 1
}

func showDialog(title, content string) {
	a := app.New()
	w := a.NewWindow("提示")
	dialog.ShowInformation(title, content, w)
}

func toInterfaceSlice(ms config.Machines) []interface{} {
	result := make([]interface{}, len(ms))
	for i, m := range ms {
		result[i] = m
	}
	return result
}

func contains(slice []int, value int) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}
