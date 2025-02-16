package executor

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/nxtcoder17/go.pkgs/log"
)

/*
SSE executor is for server-sent events executor,
any client can connect to this event at /event
*/

type SSEExectuor struct {
	ch     chan Event
	server *http.Server

	logger log.Logger
}

// OnWatchEvent implements Executor.
func (s *SSEExectuor) OnWatchEvent(event Event) error {
	select {
	case s.ch <- event:
		return nil
	case <-time.After(20 * time.Millisecond):
		s.logger.Warn("SSE event is being ignored")
		return nil
	}
}

// Start implements Executor.
func (s *SSEExectuor) Start() error {
	s.logger.Info("Server Side Event notifier server started", "addr", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
	return nil
}

// Stop implements Executor.
func (s *SSEExectuor) Stop() error {
	return s.server.Close()
}

var _ Executor = (*SSEExectuor)(nil)

type SSEExecutorArgs struct {
	Addr string

	Logger log.Logger
}

func NewSSEExecutor(args SSEExecutorArgs) *SSEExectuor {
	ch := make(chan Event)

	mux := http.NewServeMux()

	logger := args.Logger
	if logger == nil {
		logger = log.New(log.Options{})
	}

	mux.HandleFunc("/event", func(w http.ResponseWriter, req *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			logger.Error(fmt.Errorf("failed to create http.Flusher, can not use SSE"), "")
		}
		for {
			event := <-ch
			b, err := json.Marshal(event)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

			// INFO: without \n http streaming will not work
			fmt.Fprintf(w, "%s\n", b)
			flusher.Flush()
		}
	})

	server := http.Server{
		Addr:    args.Addr,
		Handler: mux,
	}

	return &SSEExectuor{ch: ch, server: &server, logger: logger}
}
