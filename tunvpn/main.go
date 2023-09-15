package main

import (
	"fmt"
	buf "github.com/liuyehcf/common-gtools/buffer"
	"k8s.io/klog/v2"
	"log"
	"net"
	"os"
	"strconv"
	"syscall"
	"time"
	"tunvpn/tun"
	"tunvpn/util"
)

var (
	tunD *tun.TunIf

	fd int

	err error

	// tcp tunnel's ip and port
	peerIp   net.IP
	peerPort int

	// ip in current side
	tunIp net.IP

	// virtual network
	tunNet *net.IPNet

	c chan struct{}
)

func main() {
	parseTunIp()
	initTun()
	defer func() {
		c <- struct{}{}
	}()
	go tunD.TunReceiveLoop()
	go HandleReceiveFromTun()
	go tcpListenerLoop()
	go tunD.TunWriteLoop()
	<-make(chan interface{})
}

func parseTunIp() {
	peerIp = net.ParseIP(os.Args[1]).To4()

	peerPort, err = strconv.Atoi(os.Args[2])

	tunIp, tunNet, err = net.ParseCIDR(os.Args[3])

	tunIp = tunIp.To4()

	log.Printf("tunIp='%s'", tunIp.String())
}

func initTun() {
	// create a tun handler
	tunD, err = tun.NewTunIf("edgemeshTun", tunIp)
	if err != nil {
		klog.Errorf("create tun device err: ", err)
	}
	err = tunD.SetupTunDevice()
	if err != nil {
		klog.Errorf("tun dev setup err: ", err)
	}
	err = tunD.AddRouteToTun(tunNet.String())
	if err != nil {
		klog.Errorf("tun Route add err: ", err)
	}
	tunD.Fd, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	if err != nil {
		klog.Errorf("tun Socket fd get err: ", err)
	}
}

func HandleReceiveFromTun() {
	//buffer := buf.NewRecycleByteBuffer(65536)
	//c0, c1 := net.Pipe()
	stream := tcpSendLoop()
	//packet := make([]byte, 65536)
	for bytes := range tunD.ReceivePipe {
		_, err := stream.Write(bytes)
		if err != nil {
			return
		}
	}
	/*for {
		select {
		case <-c:
			klog.Infof("Close HandleReceive Process")
			return
		case packet = <-tunD.ReceivePipe:
			klog.Infof("Get Tun Receive %d", len(tunD.ReceivePipe))

			n, err := c1.Write(packet)
			if err != nil {
				klog.Errorf("Error writing data: %v\n", err)
				return
			}
			buffer.Write(packet[:n])
			frame, err := util.ParseIPFrame(buffer)
			klog.Infof("frame IP is :", frame.GetTargetIP())
			if err != nil {
				klog.Errorf("l3 adapter get proxy stream from %s error: %w", frame.GetSourceIP(), err)
				return
			}
			go util.ProxyConn(stream, c0)
			klog.Infof("Success proxy to %v", tunD)
		}
	}*/

}

func Dial() net.Conn {
	// create a tun handler
	c0, c1 := net.Pipe()
	go func() {
		buffer := buf.NewRecycleByteBuffer(65536)
		packet := make([]byte, 65536)
		// TODO: improve the following double for logic
		for {
			// read from tun Dev
			n, _ := c0.Read(packet)
			// get data from tun
			buffer.Write(packet[:n])
			for {
				// Get IP frame to byte data to encapsulate
				frame, err := util.ParseIPFrame(buffer)
				klog.Infof("Start Receive From Tunnel,IPframe is:%s", frame.GetTargetIP())
				if err != nil {
					klog.Errorf("Parse frame failed:", err)
					buffer.Clean()
					break
				}
				if frame == nil {
					break
				}

				// transfer data to libP2P
				tunD.WritePipe <- frame.ToBytes()
				// print out the reception data
				klog.Infof("receive from tun, send through tunnel , source %s target %s len %d", frame.GetSourceIP(), frame.GetTargetIP(), frame.GetPayloadLen())
			}
		}
	}()
	return c1
}

func tcpSendLoop() net.Conn {
	var err error

	// create TCP Server simulate as libP2P server
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", peerIp, peerPort))
	klog.Infof("ip:\n", peerIp)

	if err != nil {
		klog.Errorf("create tcp addr failed", err)
	}

	var conn *net.TCPConn

	log.Println("try to connect peer")

	conn, err = net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		klog.Error("connec to %s:%d failed", peerIp, peerPort)
	}

	for {
		if err == nil {
			log.Println("connect peer success")
			break
		}

		log.Printf("try to reconnect 1s later, addr=%s, err=%v", tcpAddr.String(), err)

		time.Sleep(time.Second)

		conn, err = net.DialTCP("tcp", nil, tcpAddr)
	}

	return conn
}

func tcpListenerLoop() {
	var err error

	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", "0.0.0.0", peerPort))
	klog.Infof("ip: %s", tcpAddr)
	klog.Errorf("failed to parse tcpAddr", err)

	tcpListener, err := net.ListenTCP("tcp", tcpAddr)
	klog.Errorf("failed to listener", err)

	log.Printf("listener on '%s'\n", tcpAddr.String())
	streamConn, err := tcpListener.AcceptTCP()
	if err != nil {
		klog.Errorf("failed to accept", err)
	}

	klog.Infof(" Start to Dial")
	//tunConn := Dial()

	packet := make([]byte, 65536)
	// TODO: improve the following double for logic
	for {
		// read from tun Dev
		_, err2 := streamConn.Read(packet)
		if err2 != nil {
			return
		}
		// get data from tun

		tunD.WritePipe <- packet

	}
	//go util.ProxyConn(streamConn, tunConn)
}
