// Program server reproduces what appears to be a tsnet connection issue.
//
// Usage:
//
// - Start the server on clean state:
//
//	rm -fr -- repro.state
//	TS_AUTHKEY="<insert auth key here>" go run ./server -hostname repro-test -dir repro.state -port 31337
//
// Use the hostname "localhost" to bypass tsnet and use the net package.
// This is useful for verifying that the expected outcome works with net.
//
// - Connect to the server:
//
//	nc repro-test 31337
//
// Expected: You should be able to type lines of text into nc and get "OK"
// responses back.
//
// Observed: The connection from the client closes shortly after establishment.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"tailscale.com/tsnet"
)

var (
	hostname = flag.String("hostname", "", "Tailscale hostname to use (or localhost for comparison)")
	stateDir = flag.String("dir", "", "State directory")
	useNet   = flag.Bool("net-listen", false, "Use net.Listen instead of tsnet.Listen")
	port     = flag.Int("port", 31337, "Service port")
)

func main() {
	flag.Parse()
	switch {
	case *hostname == "":
		log.Fatal("You must provide a Tailscale -hostname to use")
	case *port == 0:
		log.Fatal("You must set a non-zero -port to listen on")
	}

	lg := log.New(os.Stdout, "[repro] ", 0)
	var listen func(string, string) (net.Listener, error)
	if *hostname == "localhost" {
		log.Print("Hostname is localhost; bypassing tsnet")
		listen = net.Listen
	} else {
		log.Printf("Hostname is %q; starting tsnet...", *hostname)
		srv := &tsnet.Server{
			Hostname: *hostname,
			Dir:      *stateDir,
			Logf:     lg.Printf,
		}
		listen = srv.Listen
		defer func() { log.Printf("Server close: %v", srv.Close()) }()
	}

	lst, err := listen("tcp", fmt.Sprintf("%s:%d", *hostname, *port))
	if err != nil {
		log.Fatalf("TS Listen: %v", err)
	}
	log.Printf("Listen %q OK", lst.Addr())

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	go func() { <-ctx.Done(); lst.Close() }()

	for ctx.Err() == nil {
		conn, err := lst.Accept()
		if err != nil {
			log.Printf("Accept failed: %v", err)
			break
		}
		log.Printf("Connected from %q", conn.RemoteAddr())
		sc := bufio.NewScanner(conn)
		for sc.Scan() {
			fmt.Println("->", sc.Text())
			fmt.Fprintf(conn, "OK %d\n", len(sc.Text()))
		}
		conn.Close()
	}

}
