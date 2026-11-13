//go:build !native

package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//go:embed web/dist/*
var staticAssets embed.FS

func handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/analyze", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "JSONで送信してください", 415)
			return
		}
		var q Request
		if err := decode(http.MaxBytesReader(w, r.Body, 8<<20), &q); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		result, err := analyze(q)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(result)
	})
	for _, name := range []string{"regions", "ja"} {
		mux.HandleFunc("/api/"+name, func(w http.ResponseWriter, r *http.Request) {
			data, err := assets.ReadFile("config/" + name + ".json")
			exe, _ := os.Executable()
			if external, e := os.ReadFile(filepath.Join(filepath.Dir(exe), "config", name+".json")); e == nil {
				if !json.Valid(external) {
					http.Error(w, "設定JSONを確認してください", 500)
					return
				}
				data = external
			}
			if err != nil {
				http.Error(w, "設定を読み込めません", 500)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Write(data)
		})
	}
	mux.HandleFunc("/api/sample", func(w http.ResponseWriter, r *http.Request) {
		data, _ := assets.ReadFile("sample.json")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(data)
	})
	static, _ := fs.Sub(staticAssets, "web/dist")
	mux.Handle("/", http.FileServer(http.FS(static)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil || host != "127.0.0.1" {
			http.Error(w, "ローカル接続のみ利用できます", 403)
			return
		}
		origin := r.Header.Get("Origin")
		if origin != "" && origin != "http://"+r.Host {
			http.Error(w, "外部ページからの操作は許可されていません", 403)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; object-src 'none'; frame-ancestors 'none'")
		mux.ServeHTTP(w, r)
	})
}
func main() {
	addr := flag.String("listen", "127.0.0.1:0", "ローカル待受アドレス")
	noBrowser := flag.Bool("no-browser", false, "ブラウザーを開かない")
	flag.Parse()
	host, _, err := net.SplitHostPort(*addr)
	if err != nil || host != "127.0.0.1" {
		log.Fatal("待受アドレスには127.0.0.1を指定してください")
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal("起動できません: ", err)
	}
	url := "http://" + listener.Addr().String()
	fmt.Println("MOGUGI JP オフライン戦闘分析\n日本版の実通信は未対応です。\n画面: " + url + "\n終了するにはこのウィンドウを閉じてください。")
	if !*noBrowser {
		_ = exec.Command("explorer.exe", url).Start()
	}
	server := &http.Server{Handler: handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	log.Fatal(server.Serve(listener))
}
