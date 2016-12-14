package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"util"
)

/*
	server有不同的端口
*/
func main() {
	log.SetOutput(os.Stdout)
	run(1010)
}

func check_err(err error) {
	if err != nil {
		panic(err)
	}
}

func run(server_port int) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", server_port))
	if err != nil {
		log.Println("listen err:", err)
		return
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("accept err:", err)
		} else {
			go handle_proxy(conn)
		}
	}
}

func handle_proxy(conn net.Conn) {
	r := &Req{req_conn: conn}
	defer func() {
		conn.Close()
		if err := recover(); err != nil {
			log.Printf("[%s]err:%v", r.host, err)
		}

		log.Printf("[%s]exit\n", r.host)
	}()
	r.get_req()
	r.fwd()
}

//实际上就是local收到的req的转发，可以认为是一个东西
type Req struct {
	req_conn    net.Conn
	server_conn net.Conn //真的server的链接
	buf         [2048]byte
	host        string
}

func (this *Req) get_req() {
	buf := this.buf[:]
	n, err := io.ReadAtLeast(this.req_conn, buf, 1)
	check_err(err)

	host_len := int(buf[0])
	if n < host_len {
		n1, err := io.ReadAtLeast(this.req_conn, buf[n:], host_len-n)
		check_err(err)
		buf = buf[:n+n1]
	} else {
		buf = buf[:n]
	}

	//host_len是总长
	host_len -= 1
	buf = buf[1:]
	this.host = string(buf[:host_len])

	buf = buf[host_len:]

	log.Printf("[%s]start\n", this.host)
	this.server_conn, err = net.Dial("tcp", this.host)
	check_err(err)
	//host与后面转发的内容一起过来了，那么剩下的需要转发掉
	if len(buf) > 0 {

		this.server_conn.Write(buf)
	}
}

func (this *Req) fwd() {
	go util.Fwd(this.req_conn, this.server_conn, this.buf[:])
	util.Fwd(this.server_conn, this.req_conn, nil)
}
