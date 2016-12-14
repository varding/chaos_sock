package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"util"
)

/*
	local放在本地，可以认为前面的握手是不会分包的，因此readFull是可以把协议一次全部读完
*/

func main() {
	var (
		local_port  int
		server_port int
	)
	flag.IntVar(&local_port, "lport", 1008, "local port")
	//一个协商端口，实际端口重新分配，尽量减少重复数据包
	flag.IntVar(&server_port, "sport", 12345, "server port")

	flag.Parse()

	log.SetOutput(os.Stdout)

	//先用一个常用的密码
	//这个是正常的sock
	//go run(local_port)
	//local<=>server<=>share 内部网共享方式
	run(local_port)
}

func run(local_port int) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", local_port))
	if err != nil {
		log.Panicln("listen err:", err)
		return
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalln("accept err:", err)
		} else {
			go handle_sock5(conn)
		}
	}
}

/*
http://blog.csdn.net/testcs_dn/article/details/7915505
*/

type Req struct {
	req_conn net.Conn
	fwd_conn net.Conn
	req_port string //用port作为sessionid来区分哪个连接
	buf      [2048]byte
}

func handle_sock5(conn net.Conn) {
	addr := conn.RemoteAddr().String()
	addr_parts := strings.Split(addr, ":")
	req_port := addr_parts[1]

	defer func() {
		conn.Close()
		if err := recover(); err != nil {
			log.Printf("[%s]%v", req_port, err)
		}

		log.Printf("[%s]exit req\n", req_port)
	}()

	r := Req{req_conn: conn, req_port: req_port}
	r.check_hand_shake()
	r.write_hand_shake_ack()
	host := r.parse_host()

	if len(host) == 0 {
		return
	}
	//连接fwd服务器
	r.connect_fwd(host)
	//给客户端写ack
	r.write_dst_ack() //立即回复可以减少延时，但是可能会给服务器增加没必要的数据接收

	r.fwd()
}

func check_err(err error) {
	if err != nil {
		panic(err)
	}
}

func (this *Req) check_hand_shake() {
	buf := this.buf[:]
	n, err := this.req_conn.Read(buf)
	check_err(err)
	buf = buf[:n]

	log.Printf("[%s]ver:%d,method num:%d,method:%d\n", this.req_port, buf[0], buf[1], buf[2])

	//sock4不支持远程dns解析，这儿不支持
	if buf[0] != 5 {
		panic("only support sock5")
	}

	method_num := int(buf[1])

	if method_num > 1 {
		log.Printf("[%s]other methods:%v\n", buf[3:])
	}
}

func (this *Req) write_hand_shake_ack() {
	ack := [2]byte{0x05, 0x00}
	this.req_conn.Write(ack[:])
}

/*

 */

func (this *Req) parse_host() string {
	buf := this.buf[:]
	//ver,cmd,rsv,atype,addr,port
	n, err := this.req_conn.Read(buf)
	check_err(err)
	buf = buf[:n]

	cmd := buf[1]
	atype := buf[3]
	log.Printf("[%s]ver:%d,cmd:%d,atype:%d", this.req_port, buf[0], cmd, atype)

	//剩余可用的数据
	buf = buf[4:]

	if cmd == 1 {

		host := this.read_host(atype, buf)

		log.Printf("[%s]%s", this.req_port, host)
		return host

	}
	return ""
}

func (this *Req) connect_fwd(host string) {
	var err error
	this.fwd_conn, err = net.Dial("tcp", "127.0.0.1:1010")
	check_err(err)

	//给转发服务器发送host
	var buf_len [1]byte
	buf_len[0] = byte(len(host) + 1)
	this.fwd_conn.Write(buf_len[:])
	this.fwd_conn.Write([]byte(host))
}

func (this *Req) read_host(atype byte, buf []byte) string {

	switch atype {
	case 0x01:
		ip := net.IPv4(buf[0], this.buf[1], this.buf[2], this.buf[3])
		port := binary.BigEndian.Uint16(this.buf[4:])
		return fmt.Sprintf("%s:%d", ip.String(), port)
	case 0x03:
		addr_len := int(buf[0])
		buf := buf[1:]
		port := binary.BigEndian.Uint16(buf[addr_len:])
		return fmt.Sprintf("%s:%d", string(buf[:addr_len]), port)
	case 0x04:
		return ""
	}
	return ""
}

//直接组合好，减少服务器处理判断
func (this *Req) format_host(atype byte, addr []byte, port uint16) string {
	switch atype {
	case 0x01:
		ip_addr := net.IPv4(addr[0], addr[1], addr[2], addr[3])
		return fmt.Sprintf("%s:%d", ip_addr.String(), port)
	case 0x03:
		return fmt.Sprintf("%s:%d", string(addr), port)
	case 0x04:
		//ipv6怎么做？
		return ""
	}
	return ""
}

var dst_ack = []byte{05, 00, 00, 01, 00, 00, 00, 00, 00, 00}

//var dst_ack = []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x08, 0x43}

func (this *Req) write_dst_ack() {
	//给发起请求者回复dst
	//按道理来说给req回复了，req就马上有数据过来,local也会马上转发给服务器
	//但是服务器可能链接失败，这时候给服务器的数据会全部丢弃，这个过程就多余了，因此等服务器确认链接建立完成再发送是比较合理的
	//但是这样会增加小数据包的数量，另外也增加了延时，客户端发数据和拨号其实可以并行的，这样减少了延时
	this.req_conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x08, 0x43})
}

func (this *Req) fwd() {
	go util.Fwd(this.req_conn, this.fwd_conn, this.buf[:])
	util.Fwd(this.fwd_conn, this.req_conn, nil)
}
