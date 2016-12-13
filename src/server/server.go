package main

import (
	"fmt"
	"io"
	"log"
	"net"
)

/*
	server有不同的端口
*/
func main() {
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
}

func (this *Req) get_req() {
	var buf [16]byte
	_, err := io.ReadAtLeast(conn, buf[:], 1)
	check_err(err)

	switch buf[0] {
	case 0x01:
		_, err := io.ReadAtLeast(conn, buf[:], 6)
		check_err(err)
		ip := net.IPv4(buf[0], buf[1], buf[2], buf[3])

	case 0x03:
		_, err := io.ReadAtLeast(conn, buf[:], 1)
		check_err(err)

		//长度
		_, err = io.ReadAtLeast(conn, buf[:], 1)
		check_err(err)

		_, err = io.ReadAtLeast(conn, buf[:], int(buf[0])+2)
		check_err(err)

	case 0x04:
		_, err := io.ReadAtLeast(conn, buf[:], 18)
		check_err(err)
	}
}

func (this *Req) upstream() {
	var buf [512]byte
	for {
		n, err := conn.Read(data[:])
		check_err(err)
		fmt.Println(string(data[:n]))
	}
}

func (this *Req) downstream() {
	var buf [512]byte
	for {
		n, err := conn.Read(data[:])
		check_err(err)
		fmt.Println(string(data[:n]))
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
	r := &Req{}
	r.get_req()
}
