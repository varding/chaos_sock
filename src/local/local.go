package main

import (
	"chaos"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"util"
)

/*
	local放在本地，可以认为前面的握手是不会分包的，因此readFull是可以把协议一次全部读完
	local到fwd服务器之间的链接不稳定，ping会有不少丢包，正常延时300ms
	修改使用udp发送数据，每个数据一次发送5次，收到后立即回复ack，数据包不会重复转发
*/

func main() {
	var (
		local_port int
		//server_port int
	)
	flag.IntVar(&local_port, "lport", 1008, "local port")
	//一个协商端口，实际端口重新分配，尽量减少重复数据包
	//flag.IntVar(&server_port, "sport", 12345, "server port")

	flag.Parse()

	log.SetOutput(os.Stdout)

	//先用一个常用的密码
	//这个是正常的sock
	//go run(local_port)
	//local<=>server<=>share 内部网共享方式
	run(local_port)
}

func run(local_port int) {
	var addr *net.TCPAddr
	addr, _ = net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", local_port))
	ln, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Panicln("listen err:", err)
		return
	}

	for {
		conn, err := ln.AcceptTCP()
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

var tunnel chaos.Tunnel

type Req struct {
	req_conn    *net.TCPConn
	sock5_key   chaos.Sock5Key //ip,port
	buf         [1024]byte     //不能太长，udp不能大约1500字节
	tx_pack_cnt uint32         //发送的包序号
}

func new_req(conn *net.TCPConn, addr *net.TCPAddr) *Req {
	return &Req{req_conn: conn, sock5_key: chaos.NewSock5Key(addr), tx_pack_cnt: 100}
}

func (this *Req) Write(data []byte) {
	tunnel.Write(data, &this.sock5_key, this.tx_pack_cnt)
	this.tx_pack_cnt++
}

func handle_sock5(conn *net.TCPConn) {
	addr := conn.RemoteAddr().(*net.TCPAddr)

	defer func() {
		conn.Close()
		if err := recover(); err != nil {
			log.Printf("[%v]%v", addr, err)
		}

		log.Printf("[%v]exit req\n", addr)
	}()

	r := new_req(conn, addr)
	r.sock5_proto()
	r.read_sock5()
}

func (this *Req) sock5_proto() {
	this.check_hand_shake()
	this.write_hand_shake_ack()
	host := this.parse_host()

	if len(host) == 0 {
		return
	}

	tunnel.AddReq(this.req_conn, &this.sock5_key)

	defer func() {
		//删除掉这个入口
		tunnel.DelReq(&this.sock5_key)
	}()

	//在连接服务器前给客户端写ack，这样尽可能的让req的后续请求与host一起发出去
	//立即回复可以减少延时，但是可能会给服务器增加没必要的数据接收
	this.write_dst_ack()

	//写host
	copy(this.buf[:], []byte(host))
	buf := this.buf[:len(host)]
	this.Write(buf)
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

	log.Printf("[%v]ver:%d,method num:%d,method:%d\n", this.sock5_key, buf[0], buf[1], buf[2])

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

func (this *Req) parse_host() string {
	buf := this.buf[:]
	//ver,cmd,rsv,atype,addr,port
	n, err := this.req_conn.Read(buf)

	check_err(err)
	buf = buf[:n]

	cmd := buf[1]
	atype := buf[3]
	log.Printf("[%s]ver:%d,cmd:%d,atype:%d", this.sock5_key, buf[0], cmd, atype)

	//剩余可用的数据
	buf = buf[4:]

	if cmd == 1 {

		host := this.read_host(atype, buf)

		log.Printf("[%s]%s", this.sock5_key, host)
		return host

	}
	return ""
}

// func (this *Req) connect_fwd() {
// 	var err error
// 	this.fwd_conn, err = net.Dial("tcp", "127.0.0.1:1010")
// 	check_err(err)
// }

//发送握手信息
/*
key	32bytes
iv  32bytes
host_len 1byte
host	 n
readAtLeast多等至少一个字节，这样能一次多携带一些信息，但是不要太多
*/
// func (this *Req) send_fwd_handshake(host string) {
// 	//等待至少一个字节，最多128字节的后续数据，这些一起作为握手数据发过去，这样尽量减少第一个包的长度特征
// 	buf := this.buf[:]
// 	key := buf[:32]
// 	iv := buf[32:64]
// 	data := buf[64:]
// 	data[0] = byte(len(host) + 1)
// 	n, err := io.ReadAtLeast(this.req_conn, data[1:], 1)

// 	//给转发服务器发送host
// 	//var buf_len [1]byte
// 	//buf_len[0] = byte(len(host) + 1)
// 	//this.fwd_conn.Write(buf_len[:])
// 	this.fwd_conn.Write([]byte(host))
// }

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
// func (this *Req) format_host(atype byte, addr []byte, port uint16) string {
// 	switch atype {
// 	case 0x01:
// 		ip_addr := net.IPv4(addr[0], addr[1], addr[2], addr[3])
// 		return fmt.Sprintf("%s:%d", ip_addr.String(), port)
// 	case 0x03:
// 		return fmt.Sprintf("%s:%d", string(addr), port)
// 	case 0x04:
// 		//ipv6怎么做？
// 		return ""
// 	}
// 	return ""
// }

var dst_ack = []byte{05, 00, 00, 01, 00, 00, 00, 00, 00, 00}

//var dst_ack = []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x08, 0x43}

func (this *Req) write_dst_ack() {
	//给发起请求者回复dst
	//按道理来说给req回复了，req就马上有数据过来,local也会马上转发给服务器
	//但是服务器可能链接失败，这时候给服务器的数据会全部丢弃，这个过程就多余了，因此等服务器确认链接建立完成再发送是比较合理的
	//但是这样会增加小数据包的数量，另外也增加了延时，客户端发数据和拨号其实可以并行的，这样减少了延时
	this.req_conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x08, 0x43})
}

func (this *Req) read_sock5() {
	for {
		n, err := this.req_conn.Read(this.buf[:])
		if util.ChkError(err) == 0 {
			break
		}
		this.Write(this.buf[:n])
	}
}
