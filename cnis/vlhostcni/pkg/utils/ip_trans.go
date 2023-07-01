package utils

import (
	"fmt"
	"math/big"
	"net"
)

func InetInt2Ip(ip int64) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		byte(ip>>24), byte(ip>>16), byte(ip>>8), byte(ip))
}

func InetIP2Int(ip string) int64 {
	ret := big.NewInt(0)
	ret.SetBytes(net.ParseIP(ip).To4())
	return ret.Int64()
}

func GetMaxIP(ips []string) string {
	var _ips []int64
	for _, ip := range ips {
		_ips = append(_ips, InetIP2Int(ip))
	}
	maxNum := _ips[0]
	var maxArrayIndex int
	for i := 1; i < len(_ips); i++ {
		if _ips[i] > maxNum {
			maxNum = _ips[i]
			maxArrayIndex = i
		}
	}
	return ips[maxArrayIndex]
}
