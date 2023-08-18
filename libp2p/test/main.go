package main

import (
	"fmt"

	"ideamesh/p2p/test/port"
)

func main() {
	a := port.GetPort()
	fmt.Printf("port is : %v", a)
}
