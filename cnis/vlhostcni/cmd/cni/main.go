package main

import (
	"edgemesh/vlhostcni/pkg/conf"
	"edgemesh/vlhostcni/pkg/ipam"
	"edgemesh/vlhostcni/pkg/nettools"
	"edgemesh/vlhostcni/pkg/skel"
	"encoding/json"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
	"net"

	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
)

// cmdAdd cni 标准，用于增加网络配置
func cmdAdd(args *skel.CmdArgs) error {
	/**
	 * 信息准备操作，主要是获取到配置信息，生成对应的操作对象，然后与数据库（ApiServer交互）
	 * 本阶段主要任务：
	 *		1. 获取 conf 信息：
	 * 			1. config 文件
	 * 			2. stdin(CRI)传入
	 *		2. 获取到可以分配的 IP 地址： IPAM 插件与集群编排系统交互@TODO
	 */

	// print 开始添加网络设备
	klog.Infof("======= ready to run cmdAdd =======")
	// print 输出容器信息
	klog.Infof(
		"Add ns resouces to ", "ContainerID: ", args.ContainerID,
		"\n Netns: ", args.Netns,
		"\n IfName: ", args.IfName,
		"\n Args: ", args.Args,
		"\n Path: ", args.Path,
		"\n StdinData: ", string(args.StdinData))

	// init 初始化读取配置的程序对象
	pluginConf := &conf.PluginConf{}
	// run&err 读取 stdin 的配置参数，如果失败，就输出错误信息
	// @TODO：计划增加更合理的错误处理方式--> utils
	if err := json.Unmarshal(args.StdinData, pluginConf); err != nil {
		klog.Infof("can not read config from stdin")
		return err
	}

	// print 输出读取转化的信息
	klog.Infof("edgemesh get info :", "pluginConf.Bridge", pluginConf.Bridge,
		"\n pluginConf.CNIVersion", pluginConf.CNIVersion,
		"\n pluginConf.Name", pluginConf.Name,
		"\n pluginConf.Subnet", pluginConf.Subnet,
		"\n pluginConf.Type", pluginConf.Type)

	// init 使用 kubelet(containerd) 传来的子网信息[分配的 subnet 网段]地址信息初始化 ipam
	// define ipam plugins 管理地址分配
	// @TODO： 是否单独实现这部分内容到plugins？
	ipam.Init(pluginConf.Subnet)
	ipamClient, err := ipam.GetIpamService()
	// print 如果 ipam 执行有问题
	if err != nil {
		klog.Errorf("create ipam plugins failed:", err.Error())
		return err
	}
	//run 根据 subnet 网段来得到网关, 表示所有的节点上的 pod 的 ip 都在这个网关范围内
	//@TODO：对不同的网段应该设置不同网关，划分网段是问题
	gateWay, err := ipamClient.Get().Gateway()
	// print
	if err != nil {
		klog.Errorf("get gateWay info failed:", err.Error())
		return err
	}
	//run 获得网段号
	gateWayWithMaskSegment, err := ipamClient.Get().GatewayWithMaskSegment()
	//print
	if err != nil {
		klog.Errorf(" get gateWayWithMaskSegment failed:", err.Error())
		return err
	}

	// run 获得网桥的名字,如果没有设置的话默认 egdemeshCni0
	bridgeName := pluginConf.Bridge
	if bridgeName == "" {
		bridgeName = "edgemeshCni0"
	}

	// init 设置 mtu 常数为1500
	// @TODO：vxlan,ipip,或者其他形式需要修改包大小
	// 这里如果不同节点间通信的方式使用 vxlan 的话, 这里需要变成 1460 因为 vxlan 设备会给报头中加一个 40 字节的 vxlan 头部
	mtu := 1500

	// init 获取 containerd 传过来的网卡名, 这个网卡名要被插到 net ns 中
	ifName := args.IfName
	// init 根据 containerd 传过来的 netns 的地址获取 ns
	netns, err := ns.GetNS(args.Netns)
	// print
	if err != nil {
		klog.Errorf("get ns info failed:", err.Error())
		return err
	}

	//run 从未分配的 IP 池中分配一个地址
	podIP, err := ipamClient.Get().UnusedIP()
	// print
	if err != nil {
		klog.Errorf("allocate Pod IP failed:", err.Error())
		return err
	}

	//define: 拼接 pod 的 cidr ，获取实际的 podIP = podIP + "/" + ipamClient.MaskSegment
	podIP = podIP + "/" + "24"

	/**
	 * 准备操作做完之后就可以调用网络工具来创建网络了
	 * nettools 主要做的事情:
	 *		1. 根据网桥名创建一个网桥
	 *		2. 根据网卡名儿创建一对儿 veth
	 *		3. 把叫做 IfName 的怼到 pod(netns) 上
	 *		4. 把另外一个干到主机的网桥上
	 *		5. set up 网桥以及这对儿 veth
	 *		6. 在 pod(netns) 里创建一个默认路由, 把匹配到 0.0.0.0 的 ip 都让其从 IfName 那块儿 veth 往外走
	 *		7. 设置主机的 iptables, 让所有来自 bridgeName 的流量都能做 forward(因为 docker 可能会自己设置 iptables 不让转发的规则)
	 */
	err = nettools.CreateBridgeAndCreateVethAndSetNetworkDeviceStatusAndSetVethMaster(bridgeName, gateWayWithMaskSegment, ifName, podIP, mtu, netns)
	if err != nil {
		klog.Errorf("执行创建网桥, 创建 veth 设备, 添加默认路由等操作失败, err: ", err.Error())
		err = ipamClient.Release().IPs(podIP)
		if err != nil {
			klog.Errorf("释放 podIP", podIP, " 失败: ", err.Error())
		}
	}
	/**
	 * 到这儿为止, 同一台主机上的 pod 可以 ping 通了
	 * 并且也可以访问其他网段的 ip 了
	 * 不过此时只能 ping 通主机上的网卡的网段(如果数据包没往外走的话需要确定主机是否开启了 ip_forward)
	 * 暂时没法 ping 通外网
	 * 因为此时的流量包只能往外出而不能往里进
	 * 原因是流量包往外出的时候还需要做一次 snat
	 * 没做 nat 转换的话, 外网在往回送消息的时候不知道应该往哪儿发
	 * 不过 testcni 这里暂时没有做 snat 的操作, 因为暂时没这个需求~
	 *
	 *
	 * 接下来要让不同节点上的 pod 互相通信了
	 * 可以尝试先手动操作
	 *  1. 主机上添加路由规则: ip route add 10.244.x.0/24 via 192.168.98.x dev ens33, 也就是把非本机的节点的网段和其他 node 的 ip 做个映射
	 *  2. 其他每台集群中的主机也添加
	 *  3. 把每台主机上的对外网卡都用 iptables 设置为可 ip forward: iptables -A FORWARD -i testcni0 -j ACCEPT
	 * 以上手动操作可成功
	 */

	// 首先通过 ipam 获取到 etcd 中存放的集群中所有节点的相关网络信息
	networks, err := ipamClient.Get().AllHostNetwork()
	if err != nil {
		klog.Errorf("这里的获取所有节点的网络信息失败, err: ", err.Error())
		return err
	}

	// 然后获取一下本机的网卡信息
	currentNetwork, err := ipamClient.Get().HostNetwork()
	if err != nil {
		klog.Errorf("获取本机网卡信息失败, err: ", err.Error())
		return err
	}

	// 这里面要做的就是把其他节点上的 pods 的 cidr 和其主机的网卡 ip 作为一条路由规则创建到当前主机上
	err = nettools.SetOtherHostRouteToCurrentHost(networks, currentNetwork)
	if err != nil {
		klog.Errorf("给主机添加其他节点网络信息失败, err: ", err.Error())
		return err
	}

	link, err := netlink.LinkByName(currentNetwork.Name)
	if err != nil {
		klog.Errorf("获取本机网卡失败, err: ", err.Error())
		return err
	}
	err = nettools.SetIptablesForDeviceToFarwordAccept(link.(*netlink.Device))
	if err != nil {
		klog.Errorf("设置本机网卡转发规则失败")
		return err
	}

	_gw := net.ParseIP(gateWay)

	_, _podIP, _ := net.ParseCIDR(podIP)

	result := &current.Result{
		CNIVersion: pluginConf.CNIVersion,
		IPs: []*current.IPConfig{
			{
				Address: *_podIP,
				Gateway: _gw,
			},
		},
	}
	types.PrintResult(result, pluginConf.CNIVersion)

	return nil
}

func cmdDel(args *skel.CmdArgs) error {
	klog.Errorf("进入到 cmdDel")
	klog.Errorf(
		"这里的 CmdArgs 是: ", "ContainerID: ", args.ContainerID,
		"Netns: ", args.Netns,
		"IfName: ", args.IfName,
		"Args: ", args.Args,
		"Path: ", args.Path,
		"StdinData: ", string(args.StdinData))
	// 这里的 del 如果返回 error 的话, kubelet 就会尝试一直不停地执行 StopPodSandbox
	// 直到删除后的 error 返回 nil 未知
	// return errors.New("test cmdDel")
	return nil
}

func cmdCheck(args *skel.CmdArgs) error {
	klog.Errorf("进入到 cmdCheck")
	klog.Errorf(
		"这里的 CmdArgs 是: ", "ContainerID: ", args.ContainerID,
		"Netns: ", args.Netns,
		"IfName: ", args.IfName,
		"Args: ", args.Args,
		"Path: ", args.Path,
		"StdinData: ", string(args.StdinData))
	return nil
}

func main() {
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, bv.BuildString("testcni"))
}
