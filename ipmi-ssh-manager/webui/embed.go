package webui

import "embed"

// 说明:
// 之前仅嵌入了 *.html, 在 wails build 后 style.css 与 app.js 没被打进二进制 -> 运行时加载 404,
// 导致界面丢失样式与脚本(看到白底所有区块全展开). 扩展嵌入模式包含 css/js 及 wailsjs 目录。
//go:embed *.html *.css *.js wailsjs/*
var Assets embed.FS
