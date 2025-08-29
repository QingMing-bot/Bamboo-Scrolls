package webui

import "embed"

// Assets 内嵌前端静态文件 (当前仅 index.html，可后续扩展 pattern)
//go:embed index.html
var Assets embed.FS
