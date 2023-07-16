package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/mdlayher/vxlan"
	"github.com/multiformats/go-multiaddr"
)

const vxlanProtocolID = protocol.ID("/vxlan/1.0.0")

func main() {
	// 创建一个带有超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	// 创建一个新的libp2p节点
	node, err := libp2p.New(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// 打印节点的ID和监听地址
	fmt.Println("Node ID:", node.ID())
	fmt.Println("Node addresses:", node.Addrs())

	// 设置一个处理VXLAN隧道请求的函数
	node.SetStreamHandler(vxlanProtocolID, handleVXLANStream)

	// 连接到另一个节点
	targetAddr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/4001/p2p/QmX1dd5DtkgoiYRKaPQPTCtXArUu4jEZ62rJBUcd5WhxAZ")
	targetInfo, err := peer.AddrInfoFromP2pAddr(targetAddr)
	if err != nil {
		log.Fatal(err)
	}

	err = node.Connect(ctx, *targetInfo)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to:", targetInfo.ID)

	// 创建VXLAN隧道
	stream, err := node.NewStream(ctx, targetInfo.ID, vxlanProtocolID)
	if err != nil {
		log.Fatal(err)
	}

	vxlanTunnel := createVXLAN(stream)
	defer vxlanTunnel.Close()

	// 在这里使用VXLAN隧道进行数据传输
}

// handleVXLANStream 处理来自其他节点的VXLAN隧道请求
func handleVXLANStream(stream network.Stream) {
	fmt.Println("New VXLAN stream from:", stream.Conn().RemotePeer())

	vxlanTunnel := createVXLAN(stream)
	defer vxlanTunnel.Close()

	// 在这里使用VXLAN隧道进行数据传输
}

// createVXLAN 使用libp2p stream创建一个VXLAN隧道
func createVXLAN(stream network.Stream) *vxlan.PacketConn {
	vxlanConfig := &vxlan.Config{
		Port: 4789,
	}

	vxlanTunnel, err := vxlan.NewPacketConn(vxlanConfig, &streamConn{stream: stream})
	if err != nil {
		log.Fatal(err)
	}

	return vxlanTunnel
}

// streamConn 实现了net.PacketConn接口的libp2p stream
type streamConn struct {
	stream network.Stream
}

func (c *streamConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, err = c.stream.Read(p)
	return n, c.stream.Conn().RemoteMultiaddr(), err
}

func (c *streamConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	return c.stream.Write(p)
}

func (c *streamConn) Close() error {
	return c.stream.Close()
}

func (c *streamConn) LocalAddr() net.Addr {
	return c.stream.Conn().LocalMultiaddr()
}

func (c *streamConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *streamConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *streamConn) SetWriteDeadline(t time.Time) error {
	return nil
}
