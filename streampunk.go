package main

import (
	"fmt"
	"sync"
	"strconv"
)

type InPort chan string
type OutPort chan string

type Component func(*Process)

type Process struct {
	component Component
	inputs    map[string]InPort
	outputs   map[string]OutPort
}

func NewProcess(component Component) *Process {
	proc := new(Process)
	proc.component = component
	proc.inputs = make(map[string]InPort)
	proc.outputs = make(map[string]OutPort)
	return proc
}
func (proc *Process) Input(name string) InPort {
	if proc.inputs[name] == nil {
		proc.inputs[name] = make(InPort)
	}
	return proc.inputs[name]
}
func (proc *Process) Output(name string) OutPort {
	if proc.outputs[name] == nil {
		proc.outputs[name] = make(OutPort)
	}
	return proc.outputs[name]
}
func (proc *Process) Run() {
	proc.component(proc)
	for _, port := range proc.outputs {
		close(port)
	}
}

type Network struct {
	procs []*Process
}

func NewNetwork() *Network {
	return new(Network);
}
func (net *Network) Proc(component Component) *Process {
	var proc *Process = NewProcess(component)
	net.procs = append(net.procs, proc)
	return proc
}
func (net *Network) Connect(out OutPort, in InPort) {
	go func() {
		for {
			msg, ok := <- out
			if !ok { break; }
			in <- msg
		}
		// TODO: keep track of number of connections
		close(in)
	}()
}
func (net *Network) Initialize(in InPort, value string) {
	go func() {
		in <- value
	}()
}
func (net *Network) Run() {
	var wg sync.WaitGroup
	for _, proc := range net.procs {
		wg.Add(1)
		go func(proc *Process) {
			proc.Run()
			wg.Done()
		}(proc)
	}
	wg.Wait()
}

func sender(proc *Process) {
	countStr := <- proc.Input("COUNT")
	count, _ := strconv.Atoi(countStr)
	for i := 0; i < count; i++ {
		proc.Output("OUT") <- "data: " + strconv.Itoa(i);
	}
}

func copier(proc *Process) {
	for {
		msg, ok := <- proc.Input("IN")
		if !ok { break; }
		proc.Output("OUT") <- msg
	}
}

func receiver(proc *Process) {
	for {
		msg, ok := <- proc.Input("IN")
		if !ok { break; }
		fmt.Println(msg)
	}
}

type OnDataCallback func(InPort, func(string))
type ReactiveComponent func(*Process, OnDataCallback)

func AdaptReactiveComponent(component ReactiveComponent) Component {
	return func(proc *Process) {
		var wg sync.WaitGroup
		onData := func(port InPort, callback func(string)) {
			wg.Add(1)
			go func() {
				for {
					msg, ok := <- port
					if !ok { break; }
					callback(msg)
				}
				wg.Done()
			}()
		}
		component(proc, onData);
		wg.Wait()
	}
}

func mergeReactive(proc *Process, onData OnDataCallback) {
	onData(proc.Input("IN[0]"), func(data string) {
		proc.Output("OUT") <- data
	})
	onData(proc.Input("IN[1]"), func(data string) {
		proc.Output("OUT") <- data
	})
}

func main() {
	net := NewNetwork()

	send0 := net.Proc(sender);
	send1 := net.Proc(sender);
	merge := net.Proc(AdaptReactiveComponent(mergeReactive));
	recv := net.Proc(receiver);

	net.Initialize(send0.Input("COUNT"), "1000")
	net.Initialize(send1.Input("COUNT"), "1000")

	net.Connect(send0.Output("OUT"), merge.Input("IN[0]"))
	net.Connect(send1.Output("OUT"), merge.Input("IN[1]"))
	net.Connect(merge.Output("OUT"), recv.Input("IN"))

	net.Run()
}
