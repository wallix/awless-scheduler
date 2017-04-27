package main

import (
	"errors"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/wallix/awless/logger"
	"github.com/wallix/awless/template"
	"github.com/wallix/awless/template/driver"
)

func newTemplateEnv(fn func(string) (template.Definition, bool)) *template.Env {
	env := template.NewEnv()
	env.DefLookupFunc = fn
	return env
}

func createTmpFSStore() store {
	dir, err := ioutil.TempDir("", "scheduler-")
	if err != nil {
		panic(err)
	}

	fs, err := NewFSStore(dir)
	if err != nil {
		panic(err)
	}

	return fs
}

func assertStatus(t *testing.T, resp *http.Response, expect int) {
	if got, want := resp.StatusCode, expect; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}

type happyDriver struct {
}

func (*happyDriver) Lookup(...string) (driver.DriverFn, error) {
	return func(params map[string]interface{}) (interface{}, error) { return params["name"], nil }, nil
}
func (*happyDriver) SetDryRun(bool)           {}
func (*happyDriver) SetLogger(*logger.Logger) {}

type failDriver struct {
}

func (*failDriver) Lookup(...string) (driver.DriverFn, error) {
	return func(params map[string]interface{}) (interface{}, error) {
		return nil, errors.New("mock driver failure")
	}, nil
}
func (*failDriver) SetDryRun(bool)           {}
func (*failDriver) SetLogger(*logger.Logger) {}
