// fetch 是一个极简的 HTTPS 下载/列目录工具，仅用 Go 标准库。
// 用途：本机沙箱的 schannel TLS 不可用，Go 自带纯 Go TLS 实现可以正常联网。
// 用法：
//   go run . list <url> <href正则>   —— 列出页面中匹配的 href
//   go run . dl <url> <输出文件>      —— 下载到本地文件
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"time"
)

func newClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func get(url string) (string, error) {
	client := newClient(120 * time.Second)
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: fetch list <url> <pattern> | fetch dl <url> <outfile>")
		os.Exit(2)
	}
	mode, url := os.Args[1], os.Args[2]

	if mode == "list" {
		html, err := get(url)
		if err != nil {
			fmt.Println("ERR", err)
			os.Exit(1)
		}
		re := regexp.MustCompile(`href="([^"]+)"`)
		pre := regexp.MustCompile(os.Args[3])
		seen := map[string]bool{}
		for _, m := range re.FindAllStringSubmatch(html, -1) {
			h := m[1]
			if pre.MatchString(h) && !seen[h] {
				seen[h] = true
				fmt.Println(h)
			}
		}
		return
	}

	if mode == "head" {
		req, err := http.NewRequest("HEAD", url, nil)
		if err != nil {
			fmt.Println("ERR", err)
			os.Exit(1)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36")
		client := newClient(30 * time.Second)
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("ERR", err)
			os.Exit(1)
		}
		resp.Body.Close()
		fmt.Println("STATUS", resp.StatusCode, "LEN", resp.ContentLength)
		return
	}

	if mode == "dl" || mode == "probe" {
		out := os.Args[3]
		client := newClient(0) // 大文件不限时
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			fmt.Println("ERR", err)
			os.Exit(1)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36")
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("ERR", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		fmt.Println("STATUS", resp.StatusCode, "LEN", resp.ContentLength)
		if resp.StatusCode != 200 {
			os.Exit(1)
		}
		if mode == "probe" {
			io.Copy(io.Discard, resp.Body)
			fmt.Println("PROBE_OK")
			return
		}
		f, err := os.Create(out)
		if err != nil {
			fmt.Println("ERR", err)
			os.Exit(1)
		}
		n, err := io.Copy(f, resp.Body)
		f.Close()
		if err != nil {
			fmt.Println("ERR", err)
			os.Exit(1)
		}
		fmt.Println("OK", n)
		return
	}

	fmt.Println("unknown mode", mode)
	os.Exit(2)
}
