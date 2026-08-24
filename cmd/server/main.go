package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"museum-desalination/internal/archive"
	"museum-desalination/internal/transport"
	"museum-desalination/internal/workflow"
)

func main() {
	addr := flag.String("addr", "", "监听地址")
	self := flag.Bool("self-check", false, "运行自检并退出")
	flag.Parse()
	a := archive.NewStore(".data")
	if *self {
		a = archive.NewStore("")
	} else if err := a.Load(); err != nil {
		fmt.Fprintln(os.Stderr, "加载持久化数据失败:", err)
		os.Exit(1)
	}
	wf := workflow.New(a)
	api := transport.New(wf)
	listenAddr := resolveAddr(*addr)
	if isWildcardAddr(listenAddr) {
		fmt.Fprintln(os.Stderr, "拒绝绑定非回环通配地址:", listenAddr)
		os.Exit(1)
	}
	if *self {
		if err := selfCheck(api.Routes(), listenAddr); err != nil {
			fmt.Fprintln(os.Stderr, "自检失败:", err)
			os.Exit(1)
		}
		fmt.Println("自检通过：批次已完成创建、观测、双人复核和归档")
		return
	}
	server := &http.Server{Addr: listenAddr, Handler: api.Routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	fmt.Println("文物脱盐保护闭环服务监听", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
