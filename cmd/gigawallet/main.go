package main

import (
	"fmt"
	"os"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

func main() {
	conf := giga.LoadConfig(os.Args[1])
	fmt.Println(conf)
}
