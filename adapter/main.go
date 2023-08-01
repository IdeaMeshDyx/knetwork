package main

import (
	"fmt"

	"github.com/coreos/go-iptables/iptables"
	"github.com/spf13/viper"
)

func main() {
	// 初始化iptables
	ipt, err := iptables.New()
	if err != nil {
		fmt.Println("Error initializing iptables: ", err)
		return
	}

	// 在 nat table 创建EdgeMesh链
	err = ipt.NewChain("nat", "EdgeMesh")
	if err != nil {
		fmt.Println("Error creating EdgeMesh chain: ", err)
		return
	}

	// 插入规则，在 PREROUTING 时候将目标地址是edge网段的数据包都拦截转发到应用层的进程
	ruleSpec := []string{"-s", edgeIP + "/" + edgeMask, "-j", "DNAT", "--to-destination", appIP + ":" + appPort}
	err = ipt.Append("nat", "PREROUTING", ruleSpec...)
	if err != nil {
		fmt.Println("Error inserting rule: ", err)
		return
	}

	fmt.Println("EdgeMesh chain created and rule inserted successfully.")
}

type Adapter interface {
	// 配置文件中获取云边的网段地址
	getCIDR() (cidr, error)

	// 获取 EdgeTunnel 的端口
	// @TODO 创建独立的 EDGETUNNEL
	getTunnel() (string, error)

	// 创建 EDGEMESH 链 并依据Tunnel 信息插入转发规则
	applyRules() (bool, error)

	// 监视表中的规则，如果 Tunnel 或者 Config 文件发生修改，立即修改
	watchRules() error

	// 修改（增加/删除）表中拦截到 Tunnel 的规则
	updateRules() error
}

type cidr struct {
	cloud []string
	edge  []string
}

// 从配置文件中获取不同网段的地址
func getCIDR() cidr {
	// 读取配置文件
	// TODO： 接入到 EdgeMesh 中，需要商量这个配置文件的位置
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Println("Error reading config file: ", err)
		return nil
	}

	edgeIP := viper.GetString("edge.ip")
	cloudIP := viper.GetString("cloud.ip")

	c := &cidr{
		cloud: cloudIp,
		edge:  edgeIP,
	}
	fmt.Printf("get ip info: edge %s/%s, edgemesh info: %s:%s ", edgeIP, edgeMask, appIP, appPort)

	return c
}
