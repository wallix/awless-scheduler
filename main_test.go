package main

import (
	"net/http/httptest"
	"testing"

	"github.com/wallix/awless-scheduler/client"
	"github.com/wallix/awless/template"
	"github.com/wallix/awless/template/driver"
)

func TestTasksAPI(t *testing.T) {
	taskStore = createTmpFSStore()
	defer taskStore.Destroy()

	tserver := httptest.NewServer(routes())
	defer tserver.Close()

	driversFunc = func(region string) (driver.Driver, error) {
		return &happyDriver{}, nil
	}

	schedClient, err := client.New(tserver.URL)
	if err != nil {
		t.Fatal(err)
	}

	postTemplate := func(t *testing.T, txt string) {
		if err := schedClient.Post(client.Form{
			Region:   "us-west-1",
			RunIn:    "2m",
			RevertIn: "2h",
			Template: txt,
		}); err != nil {
			t.Fatal(err)
		}
	}

	tplText := "create user name=toto\ncreate user name=tata"

	t.Run("ping service", func(t *testing.T) {
		if err := schedClient.Ping(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("template successfully received", func(t *testing.T) {
		defer taskStore.Cleanup()

		postTemplate(t, tplText)

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

		postTemplate(t, tplText)

		tasks, err := schedClient.List()
		if err != nil {
			t.Fatal(err)
		}

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

		postTemplate(t, tplText)

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

		postTemplate(t, tplText)

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
