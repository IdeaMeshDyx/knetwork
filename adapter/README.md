# 路由转发的代码实现

通过读取配置文件当中的网络区段，将数据包拦截到应用层的应用当中 ===》 目的

经过一段时间的调研和测试，决定在 "https://pkg.go.dev/github.com/coreos/go-iptables" 基础上来开发

# 测试环境以及条件
一期目前实现能够将云边数据转发的adapter


## 实现 iptables 的架子和主调用函数

### iptables 调用架子

参考 flannel, kilo， raven 的实现，基于 "https://pkg.go.dev/github.com/coreos/go-iptables" 做一层封装

将主要的几个调用函数拿出来并封装在 iptables.go 

## adapter 的主要运行逻辑
