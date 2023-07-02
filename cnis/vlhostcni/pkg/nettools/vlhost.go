package nettools

import (
	"crypto/rand"
	"edgemesh/vlhostcni/pkg/ipam"
	"errors"
	"fmt"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/coreos/go-iptables/iptables"
	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
	"net"
	"os"
)

// CreateBridge 创建虚拟网桥
func CreateBridge(brName, gw string, mtu int) (*netlink.Bridge, error) {
	// 获取网桥对象
	l, err := netlink.LinkByName(brName)

	br, ok := l.(*netlink.Bridge)
	if ok && br != nil {
		return br, nil
	}

	// 创建新的网桥并传入参数
	br = &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name:   brName,
			MTU:    mtu,
			TxQLen: -1,
		},
	}

	// 添加虚拟网桥
	err = netlink.LinkAdd(br)
	if err != nil {
		klog.Errorf("create bridge failed", brName, "err: ", err.Error())
		return nil, err
	}

	// 这里需要通过 netlink 重新获取网桥
	// 否则光创建的话无法从上头拿到其他属性
	l, err = netlink.LinkByName(brName)

	br, ok = l.(*netlink.Bridge)
	if !ok {
		klog.Errorf("find dev but it is not bridge")
		return nil, fmt.Errorf("find %q but it is not a bridge", brName)
	}

	// 给网桥绑定 ip 地址, 让网桥作为网关
	// gw 的地址是从 IPAM 那里获取到的
	ipaddr, ipnet, err := net.ParseCIDR(gw)
	if err != nil {
		klog.Errorf("can not parse gw CIDR, err: ", err.Error())
		return nil, fmt.Errorf("gatewayIP parse failed %q: %v", gw, err)
	}
	// 将解析的地址拿出来并把这个地址绑定到网桥上
	ipnet.IP = ipaddr
	addr := &netlink.Addr{IPNet: ipnet}
	if err = netlink.AddrAdd(br, addr); err != nil {
		klog.Errorf("add gw IP to bridge failed, err: ", err.Error())
		return nil, fmt.Errorf("can not add IP %q to bridge %q, err: %v", addr, brName, err)
	}

	// 然后还要把这个网桥给 up 起来
	if err = netlink.LinkSetUp(br); err != nil {
		klog.Errorf("setup bridge failed, err: ", err.Error())
		return nil, fmt.Errorf("setup bridge %q failed, err: %v", brName, err)
	}
	return br, nil
}

// SetUpVeth 激活 veth 设备，传入的参数是 veth 列表
func SetUpVeth(veth ...*netlink.Veth) error {
	for _, v := range veth {
		// 启动 veth 设备
		err := netlink.LinkSetUp(v)
		if err != nil {
			klog.Errorf("setup veth %s failed, err: ", v.Name, err.Error())
			return err
		}
	}
	return nil
}

// CreateVethPair 创建 veth 对，初始会将一端放入到主机网络
func CreateVethPair(ifName string, mtu int) (*netlink.Veth, *netlink.Veth, error) {
	vethPairName := ""
	// @TODO：生成的 veth 对名称哈希或者是其他需求修改
	for {
		_vname, err := RandomVethName()
		vethPairName = _vname
		if err != nil {
			klog.Errorf("create name for veth pair failed, err: ", err.Error())
			return nil, nil, err
		}
		// 将名字贴到 veth 对象里面
		_, err = netlink.LinkByName(vethPairName)
		if err != nil && !os.IsExist(err) {
			// 上面生成随机名字可能会重名, 所以这里先尝试按照这个名字获取一下
			// 如果没有这个名字的设备, 那就可以 break 了
			break
		}
	}

	// 测试是否完成绑定
	if vethPairName == "" {
		return nil, nil, errors.New("generate veth pair name failed")
	}

	// 创建 veth 对象
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: ifName,
			// Flags:     net.FlagUp,
			MTU: mtu,
			// Namespace: netlink.NsFd(int(ns.Fd())), // 先不设置 ns, 要不一会儿下头 LinkByName 时候找不到
		},
		PeerName: vethPairName,
		// PeerNamespace: netlink.NsFd(int(ns.Fd())),
	}

	// 创建 veth pair
	err := netlink.LinkAdd(veth)

	if err != nil {
		klog.Errorf("create veth failed, err: ", err.Error())
		return nil, nil, err
	}

	// 尝试重新获取 veth 设备看是否能成功
	veth1, err := netlink.LinkByName(ifName) // veth1 一会儿要在 pod(net ns) 里
	if err != nil {
		// 如果获取失败就尝试删掉
		netlink.LinkDel(veth1)
		klog.Errorf("get created veth1 failed, err: ", err.Error())
		return nil, nil, err
	}

	// 尝试重新获取 veth 设备看是否能成功
	veth2, err := netlink.LinkByName(vethPairName) // veth2 在主机上
	if err != nil {
		// 如果获取失败就尝试删掉
		netlink.LinkDel(veth2)
		klog.Errorf("get created veth2 failed, err: ", err.Error())
		return nil, nil, err
	}

	return veth1.(*netlink.Veth), veth2.(*netlink.Veth), nil
}

func SetIpForVeth(veth *netlink.Veth, podIP string) error {
	// 给 veth1 也就是 pod(net ns) 里的设备添加上 podIP
	ipaddr, ipnet, err := net.ParseCIDR(podIP)
	if err != nil {
		klog.Errorf("convert podIP to ns failed ", podIP, " err: ", err.Error())
		return err
	}
	ipnet.IP = ipaddr
	err = netlink.AddrAdd(veth, &netlink.Addr{IPNet: ipnet})
	if err != nil {
		klog.Errorf("attach veth with podIP failed, podIP is: ", podIP, " err: ", err.Error())
		return err
	}

	return nil
}

func SetVethToBridge(veth *netlink.Veth, br *netlink.Bridge) error {
	// 把 veth2 加入到 br 上, veth1 不用, 因为在创建的时候已经就是插入到 ns 里头了
	// veth 创建的时候 veth2 都是在本机当中，需要之后移动他到对应的网桥或者是容器
	err := netlink.LinkSetMaster(veth, br)
	if err != nil {
		klog.Errorf("attach veth2 to bridge failed, err: ", err.Error())
		return fmt.Errorf("attach veth %q to bridge %v failed, err: %v", veth.Attrs().Name, br.Attrs().Name, err)
	}
	return nil
}

func SetVethNsFd(veth *netlink.Veth, ns ns.NetNS) error {
	err := netlink.LinkSetNsFd(veth, int(ns.Fd()))
	if err != nil {
		return fmt.Errorf("attach veth %q to netns failed: %v", veth.Attrs().Name, err)
	}
	return nil
}

func SetVethMaster(veth *netlink.Veth, br *netlink.Bridge) error {
	err := netlink.LinkSetMaster(veth, br)
	if err != nil {
		return fmt.Errorf("把 veth %q 干到网桥上失败: %v", veth.Attrs().Name, err)
	}
	return nil
}

func SetDeviceMaster(device *netlink.Device, br *netlink.Bridge) error {

	if device == nil {
		return nil
	}

	if br == nil {
		return nil
	}

	deviceMaster := device.Attrs().MasterIndex

	brIndex := br.Index

	if deviceMaster == brIndex {
		// fmt.Println("已经将网卡添加过网桥中, 无需添加")
		return nil
	}

	err := netlink.LinkSetMaster(device, br)
	if err != nil {
		return fmt.Errorf("把 veth %q 干到网桥上失败: %v", device.Attrs().Name, err)
	}
	return nil
}

func SetDefaultRouteToVeth(gwIP net.IP, veth netlink.Link) error {
	return AddDefaultRoute(gwIP, veth)
}

func SetOtherHostRouteToCurrentHost(networks []*ipam.Network, currentNetwork *ipam.Network) error {

	link, err := netlink.LinkByName(currentNetwork.Name)

	list, _ := netlink.RouteList(link, netlink.FAMILY_V4)

	if err != nil {
		return err
	}

	for _, network := range networks {
		if !network.IsCurrentHost {
			// 对于其他主机, 需要获取到其他主机的对外网卡 ip, 以及它的 pods 们所占用的网段的 cidr
			// 然后用这个 cidr 和这个 ip 做一个路由表的映射
			if link == nil {
				return err
			}

			_, cidr, err := net.ParseCIDR(network.CIDR)
			if err != nil {
				return err
			}

			isSkip := false
			for _, l := range list {
				if l.Dst != nil && l.Dst.String() == network.CIDR {
					isSkip = true
					break
				}
			}

			if isSkip {
				// fmt.Println(network.CIDR, " 已存在路由表中, 直接跳过")
				continue
			}

			ip := net.ParseIP(network.IP)

			err = AddHostRoute(cidr, ip, link)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// AddRoute forked from plugins/pkg/ip/route_linux.go
func AddRoute(ipn *net.IPNet, gw net.IP, dev netlink.Link) error {
	return netlink.RouteAdd(&netlink.Route{
		LinkIndex: dev.Attrs().Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       ipn,
		Gw:        gw,
	})
}

// AddHostRoute forked from plugins/pkg/ip/route_linux.go
func AddHostRoute(ipn *net.IPNet, gw net.IP, dev netlink.Link) error {
	return netlink.RouteAdd(&netlink.Route{
		LinkIndex: dev.Attrs().Index,
		// Scope:     netlink.SCOPE_HOST,
		Dst: ipn,
		Gw:  gw,
	})
}

func AddHostRouteWithVia(ipn *net.IPNet, via *netlink.Via, dev netlink.Link) error {
	return netlink.RouteAdd(&netlink.Route{
		LinkIndex: dev.Attrs().Index,
		Scope:     netlink.SCOPE_HOST,
		Dst:       ipn,
		Via:       via,
	})
}

// AddDefaultRoute forked from plugins/pkg/ip/route_linux.go
func AddDefaultRoute(gw net.IP, dev netlink.Link) error {
	_, defNet, _ := net.ParseCIDR("0.0.0.0/0")
	return AddRoute(defNet, gw, dev)
}

// RandomVethName forked from /plugins/pkg/ip/link_linux.go
// returns string "veth" with random prefix (hashed from entropy)
func RandomVethName() (string, error) {
	entropy := make([]byte, 4)
	_, err := rand.Read(entropy)
	if err != nil {
		return "", fmt.Errorf("failed to generate random veth name: %v", err)
	}

	// NetworkManager (recent versions) will ignore veth devices that start with "veth"
	return fmt.Sprintf("veth%x", entropy), nil
}

func SetIptablesForBridgeToForwardAccept(br *netlink.Bridge) error {
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		klog.Errorf("这里 NewWithProtocol 失败, err: ", err.Error())
		return err
	}
	err = ipt.Append("filter", "FORWARD", "-i", br.Attrs().Name, "-j", "ACCEPT")
	if err != nil {
		klog.Errorf("这里 ipt.Append 失败, err: ", err.Error())
		return err
	}
	return nil
}

func SetIptablesForDeviceToFarwordAccept(device *netlink.Device) error {
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		klog.Errorf("这里 NewWithProtocol 失败, err: ", err.Error())
		return err
	}
	err = ipt.Append("filter", "FORWARD", "-i", device.Attrs().Name, "-j", "ACCEPT")
	if err != nil {
		klog.Errorf("这里 ipt.Append 失败, err: ", err.Error())
		return err
	}
	return nil
}

func CreateBridgeAndCreateVethAndSetNetworkDeviceStatusAndSetVethMaster(
	brName, gw, ifName, podIP string, mtu int, netns ns.NetNS,
) error {
	// 先创建网桥
	br, err := CreateBridge(brName, gw, mtu)
	if err != nil {
		klog.Errorf("create bridge failed, err: ", err.Error())
		return err
	}

	err = netns.Do(func(hostNs ns.NetNS) error {
		// 创建一对 veth 设备
		containerVeth, hostVeth, err := CreateVethPair(ifName, mtu)
		if err != nil {
			klog.Errorf("create veth pair failed, err: ", err.Error())
			return err
		}

		// 把随机起名的 veth 那头放在主机上
		err = SetVethNsFd(hostVeth, hostNs)
		if err != nil {
			klog.Errorf("attach veth to ns failed: ", err.Error())
			return err
		}

		// 然后把要被放到 pod 中的那头 veth 标上 podIP
		err = SetIpForVeth(containerVeth, podIP)
		if err != nil {
			klog.Errorf("set veth ip failed, err: ", err.Error())
			return err
		}

		// 然后启动 veth 对
		err = SetUpVeth(containerVeth)
		if err != nil {
			klog.Errorf("setup veth pair failed, err: ", err.Error())
			return err
		}

		// 启动之后给这个 netns 设置默认路由 以便让其他网段的包也能从 veth 走到网桥
		// TODO: 实测后发现还必须得写在这里, 如果写在下面 hostNs.Do 里头会报错目标
		gwNetIP, _, err := net.ParseCIDR(gw)
		if err != nil {
			klog.Errorf("get gateWay IP failed, err:", err.Error())
			return err
		}

		// 给 pod(net ns) 中加一个默认路由规则, 该规则让匹配了 0.0.0.0 的都走上边创建的那个 container veth
		err = SetDefaultRouteToVeth(gwNetIP, containerVeth)
		if err != nil {
			klog.Errorf("SetDefaultRouteToVeth failed, err: ", err.Error())
			return err
		}

		hostNs.Do(func(_ ns.NetNS) error {
			// 重新获取一次 host 上的 veth, 因为 hostVeth 发生了改变
			_hostVeth, err := netlink.LinkByName(hostVeth.Attrs().Name)
			hostVeth = _hostVeth.(*netlink.Veth)
			if err != nil {
				klog.Errorf("get  hostVeth failed, err: ", err.Error())
				return err
			}
			// 启动它
			err = SetUpVeth(hostVeth)
			if err != nil {
				klog.Errorf("setup veth pair for bridge failed, err: ", err.Error())
				return err
			}

			// 把它塞到网桥上
			err = SetVethToBridge(hostVeth, br)
			if err != nil {
				klog.Errorf("attach veth to bridge, err: ", err.Error())
				return err
			}

			// 都完事儿之后理论上同一台主机下的俩 netns(pod) 就能通信了
			// 如果无法通信, 有可能是 iptables 被设置了 forward drop
			// 需要用 iptables 允许网桥做转发
			err = SetIptablesForBridgeToForwardAccept(br)
			if err != nil {
				return err
			}

			return nil
		})

		return nil
	})

	if err != nil {
		return err
	}
	return nil
}
