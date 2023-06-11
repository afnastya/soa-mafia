package main

import "fmt"

type ThreadSafeStdout struct {
	lines chan string
}

func NewThreadSafeStdout() *ThreadSafeStdout {
	stdout := ThreadSafeStdout{make(chan string)}
	go stdout.run()
	return &stdout
}

func (t *ThreadSafeStdout) Println(line string) {
	t.lines <- line
}

func (t *ThreadSafeStdout) Close() {
	close(t.lines)
}

func (t *ThreadSafeStdout) run() {
	for line := range t.lines {
		fmt.Println(line)
	}
}
