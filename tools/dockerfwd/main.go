// dockerfwd connects 127.0.0.1:<port> on this PC to 127.0.0.1:<port> inside a container.
// For each connection it starts `docker exec -i <container> nc 127.0.0.1 <port>` and carries the traffic over stdin/stdout.
// The port is not published on the host, so it is not exposed to the LAN
// (on Rancher Desktop/Windows a 127.0.0.1 publish is not reachable from the host).
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
			// ponytail: starts a docker exec per connection (hundreds of ms). If slow, put socat in the container and bundle everything into one stream
			cmd := exec.Command("docker", "--context", "default", "exec", "-i", container, "nc", "127.0.0.1", port)
			cmd.Stdin = c
			cmd.Stdout = c
			if err := cmd.Run(); err != nil && err != io.EOF {
				log.Print(err)
			}
		}(c)
	}
}
