# awless-scheduler

The scheduler service is a daemon service that receives templates to be ran and reverted at a later time. 

The service basically get templates, validates and stores them. Then it will check every so often when stored templates need to be executed.

# Usage

### Test

    go test ./... -v

### Run

As an http daemon:

    go build; ./awless-scheduler     # default to localhost:8082
    go build; ./awless-scheduler --hostport 0.0.0.0:9090

For higher security, as an unix sock daemon:

    go build; ./awless-scheduler --unix-sock

# Usage with the `awless` CLI

The scheduler is mostly used together with the [`awless` CLI](https://github.com/wallix/awless).

From the CLI you can run one-liner, file or remote template and specify the following flags to run your template at a later date:

- `--schedule`: indicates the CLI that this template will be send to the service instead of being scheduled.
- `--run-in`: postpone the execution waiting the `run-in` duration (using [Golang duration notation](https://golang.org/pkg/time/#ParseDuration))
- `--revert-in`: indicates when to revert this template in case it had a succesfull execution

Examples:

    awless create instance name=MyInstance --schedule --run-in 2h --revert-in 4h
    awless create instance name=MyInstance --schedule --revert-in 1d

## Client API

Get a new client

```go
cli, err := client.New("http://10.0.0.1:9090") # HTTP client
cli, err := client.NewLocal()                  # HTTP client pointing to localhost:8082
cli, err := client.NewUnixSock("./scheduler.sock") # Unix sock client pointing to localhost:8082
```

Post a template

```go
err := cli.Post(client.Form{
  Region:   "us-west-1",
  RunIn:    "2m",
  RevertIn: "2h",
  Template: txt,
})
```

List tasks

```go
tasks, err := cli.List()
```
