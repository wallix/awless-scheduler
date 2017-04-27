package main

import (
	"fmt"

	"log"

	"github.com/wallix/awless/template"
)

type event struct {
	tk  *task
	tpl *template.Template
	err error
}

func (evt *event) String() string {
	if evt.err != nil {
		return fmt.Sprintf("failure: %s for %s", evt.err, evt.tpl)
	}
	return fmt.Sprintf("success for %s", evt.tpl)
}

func collectEvents() {
	for evt := range eventc {
		log.Println(evt)
	}
}
