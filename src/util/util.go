package util

import (
//"fmt"
//"strings"
)

func ChkError(err error) int {
	// if err != nil {
	// 	if err.Error() == "EOF" {
	// 		fmt.Println("connection closed")
	// 		return 0
	// 	} else if strings.Index(err.Error(), "wsarecv: An existing connection was forcibly closed by the remote host") != -1 {
	// 		fmt.Println("forcibly closed by the remote")
	// 		return 0
	// 	}

	// 	fmt.Println("err:", err.Error())
	// 	return -1
	// }
	// return 1
	if err != nil {
		return 0
	}
	return -1
}
