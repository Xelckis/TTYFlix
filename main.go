package main

import (
	"github.com/xelckis/ttyflix/internal/audio"
	"github.com/xelckis/ttyflix/internal/ssh"
)

func main() {
	go audio.HttpServer()
	ssh.InitServer()
}
