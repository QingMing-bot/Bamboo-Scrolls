// 本地独立预览服务器: 仅用于在开发阶段实时预览 webui/index.html 与 style.css 的改动
// 用法: go run ./preview  然后浏览器访问 http://localhost:8099
// 特性:
// 1. 自动侦听 webui 目录下 index.html / style.css / app.js 更改
// 2. 通过 SSE (Server-Sent Events) 通知浏览器执行 location.reload()
// 3. 不依赖 Wails 打包流程, 更快迭代前端样式
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var watchFiles = []string{
	"webui/index.html",
	"webui/style.css",
	"webui/app.js", // 若存在则同样监听
}

// 简单的时间戳缓存避免频繁 reload (合并短时间内多次写入)
var lastModTime time.Time
var clients = make(map[chan string]struct{})

func main() {
	go watcher()

	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/style.css", serveRaw("webui/style.css", "text/css"))
	http.HandleFunc("/app.js", serveRaw("webui/app.js", "text/javascript"))
	http.HandleFunc("/reload", sseHandler)

	log.Println("[preview] 启动 http://localhost:8099  (Ctrl+C 退出)")
	if err := http.ListenAndServe(":8099", nil); err != nil {
		log.Fatal(err)
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	// 总是返回 index.html, 并注入 reload 客户端脚本
	data, err := os.ReadFile("webui/index.html")
	if err != nil {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("read index.html error: " + err.Error()))
		return
	}
	inject := []byte(`\n<!-- live reload injected -->\n<script>\nconst es=new EventSource('/reload');\nes.onmessage=e=>{ if(e.data==='reload'){ console.log('[live] reload'); location.reload(); } };\n</script>\n`)
	// 简单追加, 不做去重
	data = append(data, inject...)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func serveRaw(path string, mime string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(path)
		if err != nil {
			w.WriteHeader(404)
			_, _ = w.Write([]byte("not found"))
			return
		}
		w.Header().Set("Content-Type", mime+"; charset=utf-8")
		_, _ = w.Write(data)
	}
}

func sseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan string, 4)
	clients[ch] = struct{}{}
	defer func() {
		delete(clients, ch)
		close(ch)
	}()

	// 初次握手, 发送 ping
	fmt.Fprintf(w, "data: ping\n\n")
	flusher, _ := w.(http.Flusher)
	flusher.Flush()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

func broadcast(msg string) {
	for ch := range clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

func watcher() {
	// 轮询方式, 避免额外依赖 (可替换 fsnotify)
	for {
		modified := false
		latest := lastModTime
		for _, f := range watchFiles {
			fi, err := os.Stat(f)
			if err != nil {
				continue
			}
			if fi.ModTime().After(latest) {
				latest = fi.ModTime()
				modified = true
			}
		}
		if modified && latest.After(lastModTime) {
			lastModTime = latest
			log.Println("[preview] 变更 -> reload", latest.Format(time.RFC3339Nano))
			broadcast("reload")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// 允许从项目根目录执行, 修正相对路径
func init() {
	root, _ := os.Getwd()
	for i, f := range watchFiles {
		watchFiles[i] = filepath.Clean(filepath.Join(root, f))
	}
}
