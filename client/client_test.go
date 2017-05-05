package client

import (
	"log"
	"net"
	"net/http"
	"os"
	"testing"
)

func TestUnixSockClient(t *testing.T) {
	filename := "test-scheduler.sock"
	defer os.Remove(filename)

	addr, err := net.ResolveUnixAddr("unix", filename)
	if err != nil {
		log.Fatal(err)
	}
	l, err := net.ListenUnix("unix", addr)
	if err != nil {
		log.Fatal(err)
	}

	server := http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("[]"))
		}),
		Addr: addr.String(),
	}
	defer server.Close()

	go func() {
		server.Serve(l)
	}()

	cli := UnixSockClient(server.Addr)

	tasks, err := cli.List()
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(tasks), 0; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}
