package main

import (
	"net"
	"os"
	"strconv"
)

func resolveAddr(flagAddr string) string {
	if flagAddr != "" {
		return flagAddr
	}
	if port := os.Getenv("PORT"); port != "" {
		if _, err := strconv.Atoi(port); err == nil {
			return "127.0.0.1:" + port
		}
	}
	return "127.0.0.1:19081"
}

func isWildcardAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	return host == "" || host == "0.0.0.0" || host == "::"
}
