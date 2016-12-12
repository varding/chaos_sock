package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
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
	go run(local_port)
	//local<=>server<=>share 内部网共享方式
	run(local_port + 2)
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
	conn net.Conn
	r    *bufio.Reader
}

func handle_sock5(conn net.Conn) {
	defer func() {
		conn.Close()
		if err := recover(); err != nil {
			fmt.Println(err)
		}

		fmt.Println("exit req")
	}()
	r := Req{r: bufio.NewReader(conn), conn: conn}
	r.check_hello()
	r.say_hello()

}

func check_err(err error) {
	if err != nil {
		panic(err)
	}
}

func (this *Req) check_hello() {
	ver, err := this.r.ReadByte()
	check_err(err)
	fmt.Println("ver:", ver)

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
	this.conn.Write(ack[:])
}

/*

 */
func (this *Req) check_destination() {
	ver, err := this.r.ReadByte()
	check_err(err)

	cmd, err := this.r.ReadByte()
	check_err(err)
	//CONNECT：X’01’  BIND：X’02’  UDP ASSOCIATE：X’03’
	if cmd == 0x01 {

	}

	//rsv
	this.r.UnreadByte()

	atype, err := this.r.ReadByte()
	check_err(err)
	this.read_dst_addr(atype)

	port_h, err := this.r.ReadByte()
	check_err(err)

	port_l, err := this.r.ReadByte()
	check_err(err)

}

func (this *Req) read_dst_addr(atype byte) {
	switch atype {
	case 0x01:
		var ip [4]byte
		_, err := this.r.Read(ip[:])
		check_err(err)
	case 0x03:
		addr_len, err := this.r.ReadByte()
		check_err(err)

		addr_buf := make([]byte, addr_len)
		n, err := this.r.Read(addr_buf)
		check_err(err)

	case 0x04:
		var ipv6 [16]byte
		_, err := this.r.Read(ipv6[:])
		check_err(err)
	}
}

func (this *Req) write_dst_ack() {

}

// func hello_ack(conn net.Conn) {
// 	//data:=make([]byte,)
// 	var data [16]byte
// 	n, err := conn.Read(data[:2])
// 	//if n
// 	ver := data[0]
// 	method_num := data[1]
// 	//ver :=binary.uin
// }

func method_sel(conn net.Conn) {
	buf := []byte{0, 0, 0}
	n, err := io.ReadAtLeast(conn, buf, 3)
	if err != nil {
		log.Fatal("read method selection err:", err)
		conn.Close()
		return
	}

	/*
		   +----+----------+----------+
		   |VER | NMETHODS | METHODS  |
		   +----+----------+----------+
		   | 1  |    1     | 1 to 255 |
		   +----+----------+----------+
			其实只要确认是socks5就行了，至少提供一种方法，即使发送了多种方法，也不需要读后面了
	*/
	if buf[0] != 5 {
		log.Fatalf("err socks version:%d\n", buf[0])
		conn.Close()
		return
	}

	//如果有多余的验证方法就要读掉
	num_method := int(buf[1])
	data_len := num_method + 2
	if data_len > n {
		io.Copy(ioutil.Discard, conn)
	}

	//回复sock5版本,不需要验证
	_, err = conn.Write([]byte{5, 0})
	if err != nil {
		log.Fatalln("write method selection err:", err)
		conn.Close()
	}
}

func parse_request(conn net.Conn) {

}
