package port

import (
	"fmt"
	"k8s.io/apimachinery/pkg/util/net"
)

func getPort() int {
	// 设置端口范围
	maxPort := 60001
	portRange := net.PortRange{40000, 60000}
	// 创建一个 PortAllocator
	portAllocator := newPortAllocator(portRange)

	// 分配端口
	port, err := portAllocator.AllocateNext()
	if err != nil {
		fmt.Sprintf("Failed to allocate port: %v", err)
	}

	// 检查端口范围
	if port < 1 || port > maxPort {
		fmt.Sprintf("Allocated port %d is out of range [1, %d]", port, maxPort)
	}

	return port
}
