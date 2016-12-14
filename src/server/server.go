package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"util"
)

/*
	server有不同的端口
*/
func main() {
	run(1010)
	// conn, err := net.Dial("tcp", "www.csdn.net:80")
	// if err != nil {
	// 	fmt.Println(err)
	// }
	// conn.Write([]byte("http"))
	// buf := make([]byte, 512)
	// conn.Read(buf)
	// fmt.Println(string(buf))
}

func check_err(err error) {
	if err != nil {
		panic(err)
	}
}

func run(server_port int) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", server_port))
	if err != nil {
		log.Panicln("listen err:", err)
		return
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalln("accept err:", err)
		} else {
			go handle_proxy(conn)
		}
	}
}

//实际上就是local收到的req的转发，可以认为是一个东西
type Req struct {
	req_conn    net.Conn
	server_conn net.Conn //真的server的链接
	buf         [512]byte
}

func (this *Req) get_req() {
	buf := this.buf[:]
	n, err := io.ReadAtLeast(this.req_conn, buf, 1)
	check_err(err)
	buf = buf[:n]
	host_len := int(buf[0])
	if n < host_len {
		n1, err := io.ReadAtLeast(this.req_conn, buf[n:], host_len-n)
		check_err(err)
		buf = buf[:n+n1]
	}
	//host_len是总长
	host_len -= 1
	buf = buf[1:]
	host := string(buf[:host_len])
	buf = buf[host_len:]

	//host := string(buf[1:])
	fmt.Println(host)
	this.server_conn, err = net.Dial("tcp", host)
	check_err(err)
	//host与后面转发的内容一起过来了，那么剩下的需要转发掉
	if len(buf) > 0 {

		this.server_conn.Write(buf)
	}
	//this.server_conn.Write([]byte("http"))
}

func (this *Req) upstream() {
	buf := this.buf[:]
	for {
		n, err := this.req_conn.Read(buf)
		if util.ChkError(err) == 0 {
			this.server_conn.Close()
			break
		}
		//fmt.Println(string(buf[:n]))
		this.server_conn.Write(buf[:n])
	}
}

func (this *Req) downstream() {
	buf := make([]byte, 512)
	for {
		n, err := this.server_conn.Read(buf)
		if util.ChkError(err) == 0 {
			this.req_conn.Close()
			break
		}
		//fmt.Println(string(buf[:n]))
		this.req_conn.Write(buf[:n])
	}
}

func handle_proxy(conn net.Conn) {
	defer func() {
		conn.Close()
		if err := recover(); err != nil {
			fmt.Println(err)
		}

		fmt.Println("exit proxy")
	}()
	r := &Req{req_conn: conn}
	r.get_req()
	go r.upstream()
	r.downstream()
}
