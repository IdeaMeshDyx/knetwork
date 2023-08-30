package udp

import (
	"k8s.io/klog/v2"
	"log"
	"net"

	"ideamesh/p2p/test/udp/tunnel"
	"ideamesh/p2p/test/udp/util/net"
)

type RdpProxyOpt struct {
	//IP:PORT,用于本地监听的端口:
	LocalAddr string
	// 需要连接的节点名称:
	NodeName string
	//远程的3389端口：
	RemoteRdpPort int32
}

type RdpProxy struct {
}

func (r *RdpProxy) handleConn(rdpOpt RdpProxyOpt, conn net.Conn) {
	defer conn.Close()

	proxyOpts := tunnel.ProxyOptions{
		Protocol: "tcp",
		NodeName: rdpOpt.NodeName,
		IP:       "127.0.0.1",
		Port:     rdpOpt.RemoteRdpPort,
	}
	stream, err := tunnel.Agent.GetProxyStream(proxyOpts)
	if err != nil {
		klog.Errorf("l4 proxy get proxy stream from %s error: %w", proxyOpts.NodeName, err)
		return
	}

	klog.Infof("l4 proxy start proxy data between tcpserver %v", proxyOpts.NodeName)

	util.ProxyConn(stream, conn)

	klog.Infof("Success proxy to %v", conn)

}

func NewRdpProxy() *RdpProxy {
	r := &RdpProxy{}
	return r
}

func (r *RdpProxy) OpenNewRdpProxy(opt RdpProxyOpt) {

	srv, err := net.Listen(`udp`, opt.LocalAddr)
	if err != nil {
		klog.Errorf("listen UDP  server failed: %v", err)
	}
	for {
		conn, err := srv.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		klog.Infof("open a rdp proxy,listen addr:[%s],remote node:[%s]", opt.LocalAddr, opt.NodeName)

		go r.handleConn(opt, conn)
	}

}
