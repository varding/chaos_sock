package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
)

/*
	这个实际上是socks5的服务器端的一部分
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
	r        *bufio.Reader
	session  int
}

func handle_sock5(conn net.Conn) {
	defer func() {
		conn.Close()
		if err := recover(); err != nil {
			fmt.Println(err)
		}

		fmt.Println("exit req")
	}()

	r := Req{r: bufio.NewReader(conn), req_conn: conn}
	r.check_hello()
	r.say_hello()
	r.check_destination()
	for {
		r.handle_req()
	}
}

func check_err(err error) {
	if err != nil {
		panic(err)
	}
}

func (this *Req) check_hello() {
	ver, err := this.r.ReadByte()
	check_err(err)
	fmt.Println("method ver:", ver)

	method_num, err := this.r.ReadByte()
	check_err(err)

	fmt.Println("method num:", method_num)

	for i := 0; i < int(method_num); i++ {
		d, err := this.r.ReadByte()
		check_err(err)

		fmt.Printf("%d:%d\n", i, d)
	}
}

func (this *Req) say_hello() {
	ack := [2]byte{0x05, 0x00}
	this.req_conn.Write(ack[:])
}

/*

 */
func (this *Req) check_destination() {
	ver, err := this.r.ReadByte()
	check_err(err)
	fmt.Println("dst ver:", ver)

	cmd, err := this.r.ReadByte()
	check_err(err)

	//rsv
	rsv, err := this.r.ReadByte()
	check_err(err)
	fmt.Println("rsv:", rsv)

	atype, err := this.r.ReadByte()
	check_err(err)
	fmt.Println("atype:", atype)

	if cmd == 1 {
		addr := this.read_dst_addr(atype)

		var port [2]byte
		_, err = this.r.Read(port[:])
		check_err(err)

		fmt.Println("port:", binary.BigEndian.Uint16(port[:]))

		this.fwd_conn, err = net.Dial("tcp", "127.0.0.1:1010")
		check_err(err)

		var at [1]byte
		at[0] = atype
		this.fwd_conn.Write(at[:])
		this.fwd_conn.Write(addr)
		this.fwd_conn.Write(port[:])
	}

}

func (this *Req) read_dst_addr(atype byte) []byte {
	switch atype {
	case 0x01:
		var ip [4]byte
		_, err := this.r.Read(ip[:])
		check_err(err)
		//net.Dial("tcp", fmt.Sprintf(format, ...))
		fmt.Println("dst ip:", ip)
		return ip[:]
	case 0x03:
		addr_len, err := this.r.ReadByte()
		check_err(err)

		addr_buf := make([]byte, addr_len)
		_, err = this.r.Read(addr_buf)
		check_err(err)
		fmt.Println("addr:", string(addr_buf))
		return addr_buf
	case 0x04:
		var ipv6 [16]byte
		_, err := this.r.Read(ipv6[:])
		check_err(err)
		return ipv6[:]
	}
	return nil
}

func (this *Req) write_dst_ack() {

}

func (this *Req) handle_req() {
	var data [256]byte
	for {
		n, err := this.r.Read(data[:])
		check_err(err)
		fmt.Println(data[:n])
	}
}
