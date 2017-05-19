package client

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
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

	discoveryService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := json.Marshal(&Discovery{ServiceAddr: addr.String(), UnixSockMode: true})
		w.Write(b)
	}))
	defer discoveryService.Close()

	schedulerService := http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("[]"))
		}),
		Addr: addr.String(),
	}
	defer schedulerService.Close()

	go func() {
		schedulerService.Serve(l)
	}()

	cli, err := New(discoveryService.URL)
	if err != nil {
		t.Fatal(err)
	}

	tasks, err := cli.List()
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(tasks), 0; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}

func TestHTTPClient(t *testing.T) {
	schedulerAddr := "localhost:9096"
	discoveryService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := json.Marshal(&Discovery{ServiceAddr: "http://" + schedulerAddr, UnixSockMode: false})
		w.Write(b)
	}))
	defer discoveryService.Close()

	schedulerService := http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("[]"))
		}),
		Addr: schedulerAddr,
	}
	defer schedulerService.Close()

	go func() {
		schedulerService.ListenAndServe()
	}()

	cli, err := New(discoveryService.URL)
	if err != nil {
		t.Fatal(err)
	}

	tasks, err := cli.List()
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(tasks), 0; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}
