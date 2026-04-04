package main

import (
	"log"

	"github.com/baihua19941101/cdnManage/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatalf("application exited with error: %v", err)
	}
}
