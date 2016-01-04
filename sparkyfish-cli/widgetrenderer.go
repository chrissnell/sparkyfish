// Routines for managing a collection of termui widgets concurrently
package main

import (
	"github.com/gizak/termui"
)

type widgetRenderer struct {
	jobs map[string]termui.Bufferer
}

func newwidgetRenderer() *widgetRenderer {
	wr := widgetRenderer{}
	wr.jobs = make(map[string]termui.Bufferer)
	return &wr
}

func (wr *widgetRenderer) Add(name string, job termui.Bufferer) {
	wr.jobs[name] = job
}

func (wr *widgetRenderer) Delete(name string) {
	delete(wr.jobs, name)
}

func (wr *widgetRenderer) Render() {
	var jobs []termui.Bufferer
	for _, j := range wr.jobs {
		jobs = append(jobs, j)
	}
	termui.Render(jobs...)
}
