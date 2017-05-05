package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/wallix/awless/aws"
	"github.com/wallix/awless/aws/driver"
	"github.com/wallix/awless/template"
	"github.com/wallix/awless/template/driver"
)

var (
	hostport        = flag.String("hostport", "localhost:8082", "listening host:port for scheduler service")
	unixSockMode    = flag.Bool("unix-sock", false, "service uses local unix sock")
	tickerFrequency = flag.Duration("tick-frequency", 1*time.Minute, "ticker frequency to run executable tasks")
	debug           = flag.Bool("debug", false, "print debug messages")
)

var (
	schedulerDir            = filepath.Join(os.Getenv("HOME"), ".awless-scheduler")
	sockAddr                = filepath.Join(os.Getenv("HOME"), "awless-scheduler.sock")
	minDurationBeforeRevert = 1 * time.Minute
	stillExecutable         = -1 * time.Hour
	eventc                  = make(chan *event)

	taskStore         store
	defaultCompileEnv = awsdriver.DefaultTemplateEnv()
	driversFunc       = func(region string) (driver.Driver, error) { return aws.NewDriver(region, "") }
)

func main() {
	flag.Parse()

	var err error
	taskStore, err = NewFSStore(schedulerDir)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Starting event collector")
	go collectEvents()
	defer close(eventc)

	t := newTicker(taskStore, *tickerFrequency)
	log.Printf("Starting ticker (frequency = %s)", t.frequency)
	go t.start()
	defer t.stop()

	server := &http.Server{
		Addr:    *hostport,
		Handler: routes(),
	}
	defer server.Shutdown(context.Background())

	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, os.Kill, os.Interrupt, syscall.SIGTERM)
		log.Printf("Service terminated with %s. Cleaning up.", <-sigc)
		server.Shutdown(context.Background())
	}()

	if *unixSockMode {
		addr, err := net.ResolveUnixAddr("unix", sockAddr)
		if err != nil {
			log.Fatal(err)
		}
		l, err := net.ListenUnix("unix", addr)
		if err != nil {
			log.Fatal(err)
		}
		defer l.Close()

		server.Addr = addr.String()

		log.Printf("Starting scheduler service on %s", server.Addr)
		log.Fatal(server.Serve(l))
	} else {
		log.Printf("Starting scheduler service on %s", server.Addr)
		log.Fatal(server.ListenAndServe())
	}
}

func routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("scheduler up!"))
	})
	mux.HandleFunc("/tasks", tasks)

	return mux
}

func tasks(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		createTask(w, r)
		return
	} else if r.Method == http.MethodGet {
		listTasks(w, r)
		return
	}
	http.Error(w, "invalid method", http.StatusMethodNotAllowed)
	return
}

func listTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := taskStore.GetTasks()
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sort.Slice(tasks, func(i int, j int) bool { return !tasks[i].RunAt.Before(tasks[j].RunAt) })

	b, err := json.MarshalIndent(tasks, "", " ")
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(b)
}

func createTask(w http.ResponseWriter, r *http.Request) {
	if *debug {
		log.Println(r.URL.String())
	}
	region := r.FormValue("region")
	if region == "" {
		log.Println("missing region")
		http.Error(w, "missing region", http.StatusBadRequest)
		return
	}
	runAt, err := getTimeParam(r.FormValue("run"), time.Now().UTC())
	if err != nil {
		log.Println(err)
		http.Error(w, "invalid duration for 'run' param", http.StatusBadRequest)
		return
	}
	revertAt, err := getTimeParam(r.FormValue("revert"), time.Time{})
	if err != nil {
		log.Println(err)
		http.Error(w, "invalid duration for 'revert' param", http.StatusBadRequest)
		return
	}
	if !revertAt.IsZero() && revertAt.Sub(runAt).Seconds() < minDurationBeforeRevert.Seconds() {
		err = fmt.Errorf("revert time is less that %s before run time", minDurationBeforeRevert)
		log.Println(err)
		http.Error(w, err.Error(), http.StatusNotAcceptable)
		return
	}

	tplTxt, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		http.Error(w, "cannot read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	tpl, err := template.Parse(string(tplTxt))
	if err != nil {
		errMsg := fmt.Sprintf("cannot parse template: %s", err)
		log.Println(errMsg)
		log.Printf("body was '%s'", string(tplTxt))
		http.Error(w, errMsg, http.StatusUnprocessableEntity)
		return
	}

	_, _, err = template.Compile(tpl, awsdriver.DefaultTemplateEnv())

	if err != nil {
		errMsg := fmt.Sprintf("cannot compile template: %s", err)
		log.Println(errMsg)
		http.Error(w, errMsg, http.StatusUnprocessableEntity)
		return
	}
	d, err := driversFunc(region)
	if err != nil {
		errMsg := fmt.Sprintf("cannot init drivers for dryrun: %s", err)
		log.Println(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	if err = tpl.DryRun(d); err != nil {
		errMsg := fmt.Sprintf("cannot dryrun template: %s", err)
		log.Println(errMsg)
		http.Error(w, errMsg, http.StatusUnprocessableEntity)
		return
	}

	tk := &task{Content: string(tplTxt), RunAt: runAt, RevertAt: revertAt, Region: region}

	if err := taskStore.Create(tk); err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func getTimeParam(param string, defaultTime time.Time) (time.Time, error) {
	if param == "" {
		return defaultTime, nil
	}

	dur, err := time.ParseDuration(param)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().UTC().Add(dur), nil
}
