package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

type CustomTransport struct {
	Base http.RoundTripper
}

func (t *CustomTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Connection", "keep-alive")
	return t.Base.RoundTrip(req)
}
func CreateHTTPClient() (*http.Client, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Использовать прокси? (y/n): ")
	useProxyInput, _ := reader.ReadString('\n')
	useProxy := strings.TrimSpace(strings.ToLower(useProxyInput))

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:               tls.VersionTLS12,
			MaxVersion:               tls.VersionTLS13,
			InsecureSkipVerify:       false,
			PreferServerCipherSuites: true,
			CurvePreferences: []tls.CurveID{
				tls.CurveP256,
				tls.X25519,
			},
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			},
		},
	}

	if useProxy == "y" {
		fmt.Print("Введите тип прокси (1 - HTTP, 2 - SOCKS5): ")
		proxyTypeInput, _ := reader.ReadString('\n')
		proxyType := strings.TrimSpace(proxyTypeInput)

		fmt.Print("Введите адрес прокси (ip:порт): ")
		addr, _ := reader.ReadString('\n')
		addr = strings.TrimSpace(addr)

		fmt.Print("Введите логин (если есть, иначе Enter): ")
		user, _ := reader.ReadString('\n')
		user = strings.TrimSpace(user)

		fmt.Print("Введите пароль (если есть, иначе Enter): ")
		pass, _ := reader.ReadString('\n')
		pass = strings.TrimSpace(pass)

		var proxyURL string
		if user != "" && pass != "" {
			proxyURL = fmt.Sprintf("http://%s:%s@%s", user, pass, addr)
		} else {
			proxyURL = fmt.Sprintf("http://%s", addr)
		}

		parsedURL, err := url.Parse(proxyURL)
		if err != nil {
			LogEvent(fmt.Sprintf("Неправильный формат прокси: %v", err))
			return nil, fmt.Errorf("неправильный формат прокси: %v", err)
		}

		if proxyType == "2" {
			var auth *proxy.Auth
			if user != "" && pass != "" {
				auth = &proxy.Auth{User: user, Password: pass}
			}
			dialer, err := proxy.SOCKS5("tcp", addr, auth, proxy.Direct)
			if err != nil {
				LogEvent(fmt.Sprintf("Не удалось подключиться к SOCKS5: %v", err))
				return nil, fmt.Errorf("не удалось подключиться к SOCKS5: %v", err)
			}
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
		} else {
			transport.Proxy = http.ProxyURL(parsedURL)
		}
	}

	return &http.Client{
		Transport: &CustomTransport{Base: transport},
		Timeout:   time.Second * 10,
	}, nil
}
