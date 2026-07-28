package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"y-ui/internal/exec"
	"y-ui/internal/web"
)

func main() {
	port := flag.Int("port", 19999, "listen port")
	flag.Parse()

	// 驗證 sudo
	if err := exec.TestSudo(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: sudo test failed: %s\n(將需要輸入密碼才能執行管理操作)\n", err)
	}

	// 啟動時確保網關基礎規則在位（FORWARD + MASQUERADE）
	// 注：這裡的網卡/網段係示例，請根據實際環境修改
	_ = exec.RestoreGateway("eth0", "192.168.1.0/24")

	srv := web.NewServer()
	srv.SetStatus(exec.GetSystemStatus)

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	fmt.Printf("y-ui starting on %s\n", addr)
	fmt.Printf("Open: http://<ip>:%d/\n", *port)

	go func() {
		if err := srv.ListenAndServe(addr); err != nil {
			log.Fatalf("server: %s", err)
		}
	}()

	// 接收 Ctrl+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println("\nshutting down...")
}
