package metrics

import "github.com/project-kessel/parsec/internal/server"

type serverObserver struct {
	server.NoOpServerObserver
}

func newServerObserver() *serverObserver {
	return &serverObserver{}
}

var _ server.ServerObserver = (*serverObserver)(nil)
