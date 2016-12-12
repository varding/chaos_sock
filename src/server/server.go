package main

import (
	"fmt"
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

func handle_proxy(conn net.Conn) {
	defer func() {
		conn.Close()
		if err := recover(); err != nil {
			fmt.Println(err)
		}

		fmt.Println("exit proxy")
	}()
	var data [512]byte
	for {
		n, err := conn.Read(data[:])
		check_err(err)
		fmt.Println(string(data[:n]))
	}
}
