package tun

import (
	"fmt"
	"io"
	"net"
	"os/exec"
	"syscall"
	"time"

	buf "github.com/liuyehcf/common-gtools/buffer"
	"github.com/songgao/water"
	"k8s.io/klog/v2"
	util "tunvpn/util"
)

const (
	tunDevice   = "/dev/net/tun"
	ifnameSize  = 16
	ReceiveSize = 5
	SendSize    = 5
)

var _ net.Conn = (*TunIf)(nil)
var _ net.Addr = (*TunAddr)(nil)

type TunAddr struct {
	// Name of Tun
	tunName string

	// IP Tun device listen at
	tunIp net.IP
}

func (tun *TunAddr) Network() string {
	return tun.tunName
}

func (tun *TunAddr) String() string {
	return string(tun.tunIp)
}

type TunIf struct {
	// Tun Addr Info
	tunAddr TunAddr

	// Tun interface to handle the tun device
	tunDev *water.Interface

	// Receive pipeline for transport data to p2p
	ReceivePipe chan []byte

	// Tcp pipeline for transport data to p2p
	WritePipe chan []byte

	// filedescribtion
	Fd int
}

// NewTunIf New Tun Interface to handle Tun dev
func NewTunIf(name string, Ip net.IP) (*TunIf, error) {
	tun, err := water.New(water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{
			Name: name,
		},
	})
	if err != nil {
		klog.Errorf("create TunInterface failed:", err)
		return nil, err
	}

	// create raw socket for communication
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	if err != nil {
		klog.Errorf("failed to create raw socket", err)
		return nil, err
	}

	klog.Infof("Tun Interface Name: %s\n", name)
	return &TunIf{
		tunAddr: TunAddr{
			tunName: name,
			tunIp:   Ip,
		},
		tunDev:      tun,
		Fd:          fd,
		ReceivePipe: make(chan []byte, ReceiveSize),
		WritePipe:   make(chan []byte, SendSize),
	}, nil
}

func (tun *TunIf) Read(packet []byte) (int, error) {
	n, err := tun.tunDev.Read(packet)
	if err != nil && err != io.EOF {
		klog.Errorf("Read Data from TUN failed", err)
		return n, err
	}
	return n, nil
}

func (tun *TunIf) Write(packet []byte) (int, error) {
	// buffer to write data
	n := len(packet)
	buffer := buf.NewRecycleByteBuffer(65536)
	buffer.Write(packet[:n])
	for {
		// get IP data inside
		frame, err := util.ParseIPFrame(buffer)
		if err != nil {
			klog.Errorf("Parse frame failed:", err)
			buffer.Clean()
			return n, err
		}
		// no data left in the buffer
		if frame == nil {
			break
		}

		klog.Infof("TUN writer receive, send through raw socket, source %s target %s len %d", frame.GetSourceIP(), frame.GetTargetIP(), frame.GetPayloadLen())

		// send ip frame through raw socket
		addr := syscall.SockaddrInet4{
			Addr: util.IPToArray4(frame.Target),
		}
		// directly send to that IP
		err = syscall.Sendto(tun.Fd, frame.ToBytes(), 0, &addr)
		if err != nil {
			klog.Errorf("failed to send data through raw socket", err)
			return n, err
		}
	}
	return n, nil
}

func (tun *TunIf) Close() error {
	err := tun.tunDev.Close()
	if err != nil {
		klog.Errorf("Close Tun falied", err)
		return err
	}
	return nil
}

// LocalAddr return Local Tun Addr
func (tun *TunIf) LocalAddr() net.Addr {
	return &tun.tunAddr
}

func (tun *TunIf) RemoteAddr() net.Addr { return nil }

func (tun *TunIf) SetDeadline(t time.Time) error { return nil }

func (tun *TunIf) SetReadDeadline(t time.Time) error { return nil }

func (tun *TunIf) SetWriteDeadline(t time.Time) error { return nil }

// SetupTunDevice  with IP
func (tun *TunIf) SetupTunDevice() error {
	//dev := tun.tunDev
	addr := tun.tunAddr
	err := ExecCommand(fmt.Sprintf("ip address add %s dev %s", addr.tunIp, addr.tunName))
	if err != nil {
		return err
	}
	klog.Info("add %s dev %s succeed ", addr.tunIp, addr.tunName)

	err = ExecCommand(fmt.Sprintf("ip link set dev %s up", addr.tunName))
	if err != nil {
		return err
	}
	klog.Info("set dev %s up succeed", addr.tunName)
	return nil
}

// AddRouteToTun route actions for those CIDR
func (tun *TunIf) AddRouteToTun(cidr string) error {
	addr := tun.tunAddr

	err := ExecCommand(fmt.Sprintf("ip route add table main %s dev %s", cidr, addr.tunName))
	if err != nil {
		return err
	}
	klog.Info("ip route add table main %s dev %s succeed", cidr, addr.tunName)
	return nil
}

func ExecCommand(command string) error {
	//TODO： change this cmd to code
	klog.Infof("exec command '%s'\n", command)

	cmd := exec.Command("/bin/bash", "-c", command)

	err := cmd.Run()
	if err != nil {
		klog.Errorf("failed to execute Command %s , err:", command, err)
		return err
	}
	// check is dev setup right
	if state := cmd.ProcessState; state.Success() {
		klog.Errorf("exec command '%s' failed, code=%d", command, state.ExitCode(), err)
		return err
	}
	return nil
}

// TunReceiveLoop  receive data from inside Pods
func (tun *TunIf) TunReceiveLoop() {
	// buffer to receive data
	buffer := buf.NewRecycleByteBuffer(65536)
	packet := make([]byte, 65536)
	// TODO: improve the following double for logic
	for {
		// read from tun Dev
		n, err := tun.tunDev.Read(packet)
		if err != nil {
			klog.Error("failed to read data from tun", err)
			break
		}
		// get data from tun
		buffer.Write(packet[:n])
		for {
			// Get IP frame to byte data to encapsulate
			frame, err := util.ParseIPFrame(buffer)

			if err != nil {
				klog.Errorf("Parse frame failed:", err)
				buffer.Clean()
				break
			}
			if frame == nil {
				break
			}

			// transfer data to libP2P
			tun.ReceivePipe <- frame.ToBytes()
			// print out the reception data
			klog.Infof("receive from tun, send through tunnel , source %s target %s len %d", frame.GetSourceIP(), frame.GetTargetIP(), frame.GetPayloadLen())
		}
	}
	return
}

// TunWriteLoop  send data back to the pod
func (tun *TunIf) TunWriteLoop() {
	// buffer to write data
	buffer := buf.NewRecycleByteBuffer(65536)
	packet := make([]byte, 65536)
	for {
		// transfer data to libP2P
		//tun.TcpReceivePipe <- frame.ToBytes()
		packet = <-tun.WritePipe
		if n := len(packet); n == 0 {
			klog.Error("failed to read from tcp tunnel")
		}
		buffer.Write(packet[:len(packet)])

		for {
			// get IP data inside
			frame, err := util.ParseIPFrame(buffer)
			if err != nil {
				klog.Errorf("failed to parse ip package from tcp tunnel", err)
			}

			if err != nil {
				klog.Errorf("Parse frame failed:", err)
				buffer.Clean()
				break
			}
			if frame == nil {
				break
			}

			klog.Infof("receive from tunnel, send through raw socket, source %s target %s len %d", frame.GetSourceIP(), frame.GetTargetIP(), frame.GetPayloadLen())

			// send ip frame through raw socket
			addr := syscall.SockaddrInet4{
				Addr: util.IPToArray4(frame.Target),
			}
			// directly send to that IP
			err = syscall.Sendto(tun.Fd, frame.ToBytes(), 0, &addr)
			if err != nil {
				klog.Errorf("failed to send data through raw socket", err)
			}
		}
	}
}

// CleanTunDevice delete all the Route and change iin kernel
func (tun *TunIf) CleanTunDevice() error {
	addr := tun.tunAddr
	err := ExecCommand(fmt.Sprintf("ip link del dev %s mode tun", addr.tunName))
	if err != nil {
		klog.Errorf("Delete Tun Device  failed", err)
		return err
	}
	klog.Infof("Set dev %s down\n", addr.tunName)
	return nil
}

// CleanTunRoute Delete All Routes attach to Tun
func (tun *TunIf) CleanTunRoute() error {
	addr := tun.tunAddr

	err := ExecCommand(fmt.Sprintf("ip route flush %s", addr.tunIp))
	if err != nil {
		klog.Errorf("Delete Tun Route  failed", err)
		return err
	}
	fmt.Printf("Removed route from dev %s\n", addr.tunName)
	return nil
}

// CleanSingleTunRoute Delete Single Route attach to Tun
func (tun *TunIf) CleanSingleTunRoute(cidr string) error {
	addr := tun.tunAddr

	err := ExecCommand(fmt.Sprintf("ip route del table main %s dev %s", cidr, addr.tunName))
	if err != nil {
		klog.Errorf("Delete Tun Route  failed", err)
		return err
	}
	klog.Infof("Removed route for %s from dev %s\n", cidr, addr.tunName)
	return nil
}
