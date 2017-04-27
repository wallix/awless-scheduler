package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wallix/awless/template"
)

func TestTasksAPI(t *testing.T) {
	taskStore = createTmpFSStore()
	defer taskStore.Destroy()

	tserver := httptest.NewServer(routes())
	defer tserver.Close()

	tplText := "create user name=toto\ncreate user name=tata"

	postTemplate := func(txt string) {
		resp, er := http.Post(tserver.URL+"/tasks?region=us-west-1&run=2m&revert=2h", "application/text", strings.NewReader(txt))
		if er != nil {
			t.Fatal(er)
		}
		assertStatus(t, resp, 200)
	}

	t.Run("template successfully received", func(t *testing.T) {
		defer taskStore.Cleanup()

		postTemplate(tplText)

		tasks, err := taskStore.GetTasks()
		if err != nil {
			t.Fatal(err)
		}

		if got, want := len(tasks), 1; got != want {
			t.Fatalf("got %d, want %d", got, want)
		}
		if got, want := tasks[0].Content, tplText; got != want {
			t.Fatalf("got \n%q\nwant\n%q\n", got, want)
		}
	})

	t.Run("listing templates", func(t *testing.T) {
		defer taskStore.Cleanup()

		postTemplate(tplText)

		resp, err := http.Get(tserver.URL + "/tasks")
		if err != nil {
			t.Fatal(err)
		}
		assertStatus(t, resp, 200)

		var tasks []*task
		err = json.NewDecoder(resp.Body).Decode(&tasks)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if got, want := len(tasks), 1; got != want {
			t.Fatalf("got %d, want %d", got, want)
		}
		if got, want := string(tasks[0].Content), tplText; got != want {
			t.Fatalf("got %s, want %s", got, want)
		}
		if got, want := string(tasks[0].Region), "us-west-1"; got != want {
			t.Fatalf("got %s, want %s", got, want)
		}
	})

	t.Run("executing task", func(t *testing.T) {
		defer taskStore.Cleanup()

		postTemplate(tplText)

		tasks, err := taskStore.GetTasks()
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(tasks), 1; got != want {
			t.Fatalf("got %d, want %d", got, want)
		}

		task := tasks[0]

		env := newTemplateEnv(func(key string) (template.Definition, bool) {
			return template.Definition{ExtraParams: []string{"name", "user"}}, true
		})

		if _, err = task.execute(&happyDriver{}, env); err != nil {
			t.Fatal(err)
		}

		tasks, err = taskStore.GetTasks()
		if err != nil {
			t.Fatal(err)
		}

		if got, want := len(tasks), 1; got != want {
			t.Fatalf("got %d, want %d", got, want)
		}

		revertTplText := "delete user id=tata\ndelete user id=toto"
		if got, want := tasks[0].Content, revertTplText; got != want {
			t.Fatalf("got \n%q\nwant\n%q\n", got, want)
		}
	})

	t.Run("fail executing driver", func(t *testing.T) {
		defer taskStore.Cleanup()

		postTemplate(tplText)

		tasks, err := taskStore.GetTasks()
		if err != nil {
			t.Fatal(err)
		}

		if got, want := len(tasks), 1; got != want {
			t.Fatalf("got %d, want %d", got, want)
		}

		task := tasks[0]

		env := newTemplateEnv(func(key string) (template.Definition, bool) {
			return template.Definition{RequiredParams: []string{"name", "user"}}, true
		})
		if _, err := task.execute(&failDriver{}, env); err == nil {
			t.Fatal("expected error, got nil")
		}

		tasks, err = taskStore.GetTasks()
		if err != nil {
			t.Fatal(err)
		}

		if got, want := len(tasks), 0; got != want {
			t.Fatalf("got %d, want %d", got, want)
		}

		fails, err := taskStore.GetFailures()
		if err != nil {
			t.Fatal(err)
		}

		if got, want := len(fails), 1; got != want {
			t.Fatalf("got %d, want %d", got, want)
		}

		if got, want := fails[0].Content, tplText; got != want {
			t.Fatalf("got \n%q\nwant\n%q\n", got, want)
		}
	})
}
