package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/adler32"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wallix/awless/template"
	"github.com/wallix/awless/template/driver"
)

type task struct {
	Content  string
	RunAt    time.Time
	RevertAt time.Time
	Region   string
}

const stampLayout = "2006-01-02-15h04m05s"

func New(filePath string) (tk *task, err error) {
	tk = &task{}

	var content []byte
	content, err = ioutil.ReadFile(filePath)
	if err != nil {
		return
	}
	tk.Content = string(content)
	fileName := filepath.Base(filePath)
	name := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	splits := strings.SplitN(name, "_", 4)
	checksum, err := strconv.ParseUint(splits[0], 10, 32)
	if err != nil {
		return
	}
	if cs := adler32.Checksum([]byte(tk.Content)); uint32(checksum) != cs {
		err = fmt.Errorf("unexpected checksum for file %s. Exepcted %d", name, cs)
		return
	}
	tk.RunAt, err = time.Parse(stampLayout, splits[1])
	if err != nil {
		return
	}
	tk.RevertAt, err = time.Parse(stampLayout, splits[2])
	if err != nil {
		return
	}
	tk.Region = splits[3]

	return
}

func (tk *task) id() string {
	checksum := adler32.Checksum([]byte(tk.Content))
	return fmt.Sprintf("%d_%s_%s_%s.%s", checksum, tk.RunAt.UTC().Format(stampLayout), tk.RevertAt.UTC().Format(stampLayout), tk.Region, awlessFileExt)
}

func (tk *task) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString("{")
	jsonValue, err := json.Marshal(tk.Content)
	if err != nil {
		return nil, err
	}
	buffer.WriteString(fmt.Sprintf("\"Content\":%s,", jsonValue))
	if !tk.RunAt.IsZero() {
		jsonValue, err = json.Marshal(tk.RunAt)
		if err != nil {
			return nil, err
		}
		buffer.WriteString(fmt.Sprintf("\"RunAt\":%s,", jsonValue))
		buffer.WriteString(fmt.Sprintf("\"RunIn\":\"%s\",", time.Until(tk.RunAt)))
	}
	if !tk.RevertAt.IsZero() {
		jsonValue, err = json.Marshal(tk.RevertAt)
		if err != nil {
			return nil, err
		}
		buffer.WriteString(fmt.Sprintf("\"RevertAt\":%s,", jsonValue))
		buffer.WriteString(fmt.Sprintf("\"RevertIn\":\"%s\",", time.Until(tk.RevertAt)))
	}
	buffer.WriteString(fmt.Sprintf("\"Region\":\"%s\"", tk.Region))

	buffer.WriteString("}")
	return buffer.Bytes(), nil
}

func (tk *task) execute(d driver.Driver, env *template.Env) (executed *template.Template, err error) {
	defer func() {
		id := tk.id()
		if err != nil {
			taskStore.MarkAsFailed(id)
		} else {
			err = taskStore.Remove(id)
		}
	}()

	var tpl, compiled, revertTmp *template.Template

	if tpl, err = template.Parse(tk.Content); err != nil {
		return
	}

	if compiled, _, err = template.Compile(tpl, env); err != nil {
		return
	}

	if err = compiled.DryRun(d); err != nil {
		return
	}

	if executed, err = compiled.Run(d); err != nil {
		return
	}

	if executed.HasErrors() {
		var execErrors []string
		for _, cmd := range executed.CommandNodesIterator() {
			if cmd.CmdErr != nil {
				execErrors = append(execErrors, cmd.CmdErr.Error())
			}
		}
		err = fmt.Errorf(strings.Join(execErrors, ", "))
		return
	}

	if !tk.RevertAt.IsZero() && template.IsRevertible(executed) {
		if revertTmp, err = executed.Revert(); err != nil {
			return
		}
		revertTask := &task{RunAt: tk.RevertAt, Region: tk.Region, Content: revertTmp.String()}
		if err = taskStore.Create(revertTask); err != nil {
			return
		}
	}
	return
}
