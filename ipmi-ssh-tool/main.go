package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"github.com/QingMing-bot/ipmi-ssh-tool/ui"
)

func main() {
	// 1. 创建Fyne应用
	myApp := app.New() // ✅ 使用 := 声明变量

	// 2. 创建主窗口
	myWindow := ui.MainWindow(myApp) // 修复：只接收一个返回值

	// 3. 设置窗口属性
	myWindow.SetTitle("IPMI-SSH管理工具")
	myWindow.Resize(fyne.NewSize(800, 600)) // 设置初始尺寸

	// 4. 显示窗口并阻塞运行
	myWindow.ShowAndRun()
}
