package tun

import (
	"fmt"
	"net"
	"os/exec"
	"syscall"

	buf "github.com/liuyehcf/common-gtools/buffer"
	"github.com/songgao/water"
	"k8s.io/klog/v2"

	util "tunvpn/util"
)

const (
	tunDevice  = "/dev/net/tun"
	ifnameSize = 16
)

type tunIf struct {
	// IP Tun device listen at
	tunIp net.IP

	// Tun interface to handle the tun device
	tunDev *water.Interface

	// Tcp pipeline for transport data to p2p
	TcpRecievePipe chan []byte

	// Tcp pipeline for transport data to p2p
	TcpWritePipe chan []byte

	// filedescribtion
	fd int
}

// New Tuninterface to handle Tun dev
func NewTunIf(name string, Ip net.IP) (*tunIf, error) {
	tun, err := water.New(water.Config{
		DeviceType: water.TUN,
		Name:       name,
	})
	if err != nil {
		klog.Errorf("create TunInterface failed:", err)
		return err
	}

	// create raw socket for communication
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	if err != nil {
		klog.Errorf("failed to create raw socket", err)
		return err
	}

	klog.Infof("Tun Interface Name: %s\n", tun.Name())
	return &tunIf{
		tunIp:  Ip,
		tunDev: tun,
		fd:     fd,
	}, nil
}

// setupTunDevice with IP
func (tun *tunIf) SetupTunDevice() error {
	err := ExecCommand(fmt.Sprintf("ip address add %s dev %s", tun.tunIp.String(), tun.tunDev.Name()))
	if err != nil {
		return err
	}
	klog.Info("add %s dev %s succeed ", tun.tunIp.String(), tun.tunDev.Name())

	err = ExecCommand(fmt.Sprintf("ip link set dev %s up", tun.tunDev.Name()))
	if err != nil {
		return err
	}
	klog.Info("set dev %s up succeed", tun.tunDev.Name())
	return nil
}

// add route to tun dev for those CIDR
func (tun *tunIf) AddRouteToTun(Cidr string) error {
	err := ExecCommand(fmt.Sprintf("ip route add table main %s dev %s", cidr, tun.tunDev.Name()))
	if err != nil {
		return err
	}
	klog.Info("ip route add table main %s dev %s succeed", cidr, tun.tunDev.Name())
	return nil
}

func ExecCommand(command string) error {
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

// Tun Dev recieve data from inside Pods
func (tun *tunIf) TunRecieveLoop() {
	// buffer to recieve data
	buffer := buf.NewByteBuffer(65536)
	packet := make([]byte, 65536)
	for {

		n, err := tun.tunDev.Read(packet)
		if err != nil {
			klog.Error("failed to read data from tun", err)
			break
		}

		// read data from tun
		buffer.Write(packet[:n])
		for {
			// Add IP frame to byte data
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
			tun.TcpRecievePipe <- frame.ToBytes()

			klog.Infof("receive from tun, send through tunnel %s\n", frame.String())
		}
	}
	return
}

func (tun *tunIf) TunWriteLoop() {
	// buffer to write data
	buffer := buf.NewByteBuffer(65536)
	packet := make([]byte, 65536)
	for {
		n, err := conn.Read(packet)
		if err != nil {
			klog.Errorf("failed to read from tcp tunnel", err)
		}
		buffer.Write(packet[:n])

		for {
			// get IP data inside
			frame, err := tunnel.ParseIPFrame(buffer)
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

			klog.Infof("receive from tunnel, send through raw socket%s", frame.String())

			// send ip frame through raw socket
			addr := syscall.SockaddrInet4{
				Addr: tunnel.IPToArray4(frame.Target),
			}
			// directly send to that IP
			err = syscall.Sendto(fd, frame.ToBytes(), 0, &addr)
			assert.AssertNil(err, "failed to send data through raw socket")
			if err != nil {
				klog.Errorf("failed to send data through raw socket", err)
			}
		}
	}
}
