package main

import (
	"fmt"

	"github.com/coreos/go-iptables/iptables"
	"github.com/spf13/viper"
)

func main() {
	// 读取配置文件
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Println("Error reading config file: ", err)
		return
	}

	edgeIP := viper.GetString("edge.ip")
	edgeMask := viper.GetString("edge.mask")
	appIP := viper.GetString("app.ip")
	appPort := viper.GetString("app.port")

	// 初始化iptables
	ipt, err := iptables.New()
	if err != nil {
		fmt.Println("Error initializing iptables: ", err)
		return
	}

	// 创建EdgeMesh链
	err = ipt.NewChain("nat", "EdgeMesh")
	if err != nil {
		fmt.Println("Error creating EdgeMesh chain: ", err)
		return
	}

	// 插入规则，将目标地址是edge网段的数据包都拦截转发到应用层的进程
	ruleSpec := []string{"-s", edgeIP + "/" + edgeMask, "-j", "DNAT", "--to-destination", appIP + ":" + appPort}
	err = ipt.Append("nat", "PREROUTING", ruleSpec...)
	if err != nil {
		fmt.Println("Error inserting rule: ", err)
		return
	}

	fmt.Println("EdgeMesh chain created and rule inserted successfully.")
}
