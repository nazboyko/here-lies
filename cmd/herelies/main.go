// Command herelies serves Here Lies on port 8787: a page where a photo of a
// dead household thing becomes an obituary written by a local model.
package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nazboyko/here-lies/internal/obit"
	"github.com/nazboyko/here-lies/internal/ollama"
	"github.com/nazboyko/here-lies/internal/server"
	"github.com/nazboyko/here-lies/web"
)

func main() {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	addr := env("HERELIES_ADDR", "0.0.0.0:8787")
	client := &ollama.Client{
		BaseURL: env("HERELIES_OLLAMA", ollama.DefaultURL),
		Model:   env("HERELIES_MODEL", ollama.DefaultModel),
		HTTP:    &http.Client{},
	}

	static, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		logger.Fatal(err)
	}
	srv := &server.Server{
		Pipeline: &obit.Pipeline{Model: client},
		Health:   client,
		Static:   static,
		Log:      logger,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Fatalf("Can't listen on %s: %v. Is Here Lies already running?", addr, err)
	}
	host, port, _ := net.SplitHostPort(ln.Addr().String())

	h := client.Health(context.Background())
	fmt.Printf("Here Lies is open at http://localhost:%s\n", port)
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		for _, lan := range lanIPs() {
			fmt.Printf("On your phone (same Wi-Fi): http://%s:%s\n", lan, port)
		}
	}
	if h.Problem != "" {
		fmt.Println(h.Problem)
	} else {
		fmt.Printf("Model %s is ready (Ollama %s).\n", h.Model, h.Version)
	}

	hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		hs.Shutdown(ctx)
	}()
	if err := hs.Serve(ln); err != nil && err != http.ErrServerClosed {
		logger.Fatal(err)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// lanIPs lists the IPv4 addresses a phone on the same network can use.
func lanIPs() []string {
	var out []string
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok {
				if ip4 := ipn.IP.To4(); ip4 != nil && !ip4.IsLinkLocalUnicast() {
					out = append(out, ip4.String())
				}
			}
		}
	}
	return out
}
