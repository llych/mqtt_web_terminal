package tty

import (
	"context"
	"github.com/creack/pty"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

type Message struct {
	Type  string `json:"type,omitempty"` // data, size
	Data  string `json:"data,omitempty"`
	Rows  uint16 `json:"rows,omitempty""`
	Cols  uint16 `json:"cols,omitempty""`
	Error error  `json:"error,omitempty"`
}

type Term struct {
	input     chan Message
	output    chan Message
	tty       *os.File
	cmd       *exec.Cmd
	wg        sync.WaitGroup
	done      chan struct{}
	err       error
	shell     string
	resetChan chan struct{}
	rows      uint16
	cols      uint16
}

func (t *Term) Close() {
	close(t.done)
	t.tty.Close()
	t.cmd.Process.Kill()
	t.cmd.Process.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	go func() {
		t.wg.Wait()
		cancel()
	}()
	select {
	case <-ctx.Done():
		log.Printf("stop success")
	case <-time.After(10 * time.Second):
		log.Printf("stop timeout. exit")
	}
}

func (t *Term) SetSize(rows, cols uint16) {
	t.rows = rows
	t.cols = cols
	pty.Setsize(t.tty, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}

func (t *Term) ReadTty() {
	t.wg.Add(1)
	defer t.wg.Done()
	for {
		select {
		case <-t.done:
			log.Printf("tty read stopping...")
			return
		default:
			buf := make([]byte, 1024*20)
			read, err := t.tty.Read(buf)
			if err != nil {
				log.Printf("reading tty err: %v", err)
				t.ResetTty()
				t.output <- Message{
					Error: err,
					Data:  "reset tty",
				}
			} else {
				t.output <- Message{
					Error: err,
					Data:  string(buf[:read]),
				}
			}

		}
	}
}

func (t *Term) WriteTty() {
	t.wg.Add(1)
	defer t.wg.Done()
	for {
		select {
		case <-t.done:
			log.Printf("tty write stopping...")
			return
		case msg := <-t.input:
			switch msg.Type {
			case "data":
				_, err := t.tty.Write([]byte(msg.Data))
				if err != nil {
					log.Printf("writing to tty err: %v", err)
					t.ResetTty()
				}
			case "size":
				t.SetSize(msg.Rows, msg.Cols)
				log.Printf("set tty rows: %d cols: %d", msg.Rows, msg.Cols)
			}
		}
	}
}

func (t *Term) Input() chan Message {
	return t.input
}

func (t *Term) Output() chan Message {
	return t.output
}

func (t *Term) ResetTty() {
	select {
	case <-t.done:
		// stopping
		return
	case t.resetChan <- struct{}{}:
		defer func() {
			<-t.resetChan
		}()
		log.Println("reset tty ...")
		cmd := exec.Command(t.shell)
		cmd.Env = append(os.Environ(), "TERM=xterm")
		tty, err := pty.Start(cmd)
		if err != nil {
			log.Printf("reset tty err: %v", err)
			return
		}

		t.tty = tty
		t.cmd = cmd
		t.SetSize(t.rows, t.cols)
		log.Printf("reset tty done. tty rows: %d cols: %d", t.rows, t.cols)
	}

}

func New(shell string) (*Term, error) {
	if shell == "" {
		shell = "bash"
	}
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm")
	tty, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	term := &Term{
		input:     make(chan Message, 200),
		output:    make(chan Message, 200),
		done:      make(chan struct{}),
		tty:       tty,
		cmd:       cmd,
		wg:        sync.WaitGroup{},
		shell:     shell,
		resetChan: make(chan struct{}, 1),
	}
	go term.ReadTty()
	go term.WriteTty()

	return term, nil
}
