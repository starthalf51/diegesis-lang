// dockerfwd 는 이 PC 의 127.0.0.1:<port> 를 컨테이너 안의 127.0.0.1:<port> 로 잇는다.
// 연결마다 `docker exec -i <container> nc 127.0.0.1 <port>` 를 띄워 표준입출력으로 나른다.
// 포트를 호스트에 publish 하지 않으므로 LAN 에 노출되지 않는다
// (Rancher Desktop/Windows 는 127.0.0.1 publish 가 호스트에서 닿지 않는다).
//
//	go run ./tools/dockerfwd deploy-grafana-1 3000 3300   → http://127.0.0.1:3300
package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
)

func main() {
	if len(os.Args) != 3 && len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: dockerfwd <container> <port> [local-port]")
		os.Exit(2)
	}
	container, port, local := os.Args[1], os.Args[2], os.Args[2]
	if len(os.Args) == 4 {
		local = os.Args[3]
	}
	ln, err := net.Listen("tcp", "127.0.0.1:"+local)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("http://127.0.0.1:%s → %s:%s", local, container, port)
	for {
		c, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func(c net.Conn) {
			defer c.Close()
			// ponytail: 연결마다 docker exec 를 띄운다(수백 ms). 느리면 컨테이너에 socat 을 두고 스트림 하나로 묶는다
			cmd := exec.Command("docker", "--context", "default", "exec", "-i", container, "nc", "127.0.0.1", port)
			cmd.Stdin = c
			cmd.Stdout = c
			if err := cmd.Run(); err != nil && err != io.EOF {
				log.Print(err)
			}
		}(c)
	}
}
