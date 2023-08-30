package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"netconn/pkg"
	"time"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	l := pkg.ListenPipe()
	log.Println(`listen on pipe`)
	go func() {
		conn1, err := l.Accept()
		if err != nil {
			fmt.Printf("Create conn Failed %s", err)
		}
		go runServer(conn1)
	}()

	go func() {
		conn2, err := l.DialContext(context.Background())
		if err != nil {
			fmt.Printf("Create conn Failed %s", err)
		}
		go runClient(conn2)
	}()
	// Wait for goroutines to finish
	time.Sleep(15 * time.Second)
}
func runClient(conn net.Conn) {
	dataChan := make(chan []byte, 100)

	// 模拟持续不断地向dataChan发送数据
	for i := 0; i < 10; i++ {
		dataChan <- []byte(fmt.Sprintf("Message %d", i))
		time.Sleep(1 * time.Second)
	}
	// 在新的goroutine中从dataChan读取数据并写入到net.Conn中
	go func() {
		for data := range dataChan {
			_, err := conn.Write(data)
			if err != nil {
				fmt.Println("Error writing to connection:", err)
				return
			}
		}
	}()
	if len(dataChan) == 0 {
		conn.Close()
	}
}
func printResponse(resp *http.Response, e error) {
	if e != nil {
		log.Fatalln(e)
	}
	b, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		log.Fatalln(e)
	}
	fmt.Println(resp.Status)
	fmt.Println(string(b))
}

func runServer(conn net.Conn) {
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("Error reading from server:", err)
			break
		}
		fmt.Println("Received from server:", string(buf[:n]))
	}
}
