# Tun VPN 拦截程序实现


启动 实验环境的 docker
```shell
docker run -d --name vpn1 --mount type=bind,source=D:\Project\knetwork\tunvpn,target=/root/vpn  --cap-add=NET_ADMIN   --device /dev/net/tun   ubuntu tail -f /dev/null


docker run -d --name vpn2 --mount type=bind,source=D:\Project\knetwork\tunvpn,target=/root/vpn  --cap-add=NET_ADMIN   --device /dev/net/tun   ubuntu tail -f /dev/null
```

docker 中安装必要的工具
```shell
sudo apt install iputils net-tools utils-ping
```

启动实验环境

```shell
// 交叉编译 vpn
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build 

// 执行 vpn1 处于 172.17.0.2 虚拟地址为 192.168.17.1
./vpn 172.17.0.3  9999 192.168.17.1

// 执行 vpn2 处于 172.17.0.3 虚拟地址为 192.168.17.2
./vpn 172.17.0.2  9999 192.168.17.2

```

然后就是在 192.168.17.1 的机器上直接 ping 192.168.17.2 即可