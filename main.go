package main

import (
	"flag"
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
)

func pokerHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	_, _, active := s.Pty()
	if !active {
		wish.Fatalln(s, "no active terminal, sipping")
		return nil, nil
	}

	if state.masterConn == nil {
		state.masterConn = s
		m := masterView{
			revealed: false,
		}
		return m, []tea.ProgramOption{tea.WithAltScreen()}
	}

	return initialNameInputView(s), []tea.ProgramOption{tea.WithAltScreen()}
}

func main() {
	port := flag.Int("p", 23234, "SSH server port")
	flag.Parse()

	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf(":%d", *port)),
		wish.WithHostKeyPath(".ssh/term_info_ed25519"),
		wish.WithMiddleware(
			logging.Middleware(),
			bm.Middleware(pokerHandler),
		),
	)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("Starting SSH server on :%d\n", *port)
	fmt.Printf("Connect with: ssh -t localhost -p %d\n", *port)
	err = s.ListenAndServe()
	if err != nil {
		log.Fatalln(err)
	}
}
