/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package logtail

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/vogo/gstop"
	"github.com/vogo/logger"
)

var ErrWorkerCommandStopped = errors.New("worker command stopped")

type worker struct {
	mu      sync.Mutex
	id      string
	server  *Server
	stopper *gstop.Stopper
	dynamic bool      // command generated dynamically
	command string    // command lines
	cmd     *exec.Cmd // command object
	filters map[string]*Filter
}

func (w *worker) Write(data []byte) (int, error) {
	// copy data to avoid being update by source
	newData := make([]byte, len(data))
	copy(newData, data)

	for _, r := range w.filters {
		r.receive(newData)
	}

	_, _ = w.server.Write(newData)

	return len(newData), nil
}

func (w *worker) writeToFilters(bytes []byte) (int, error) {
	for _, r := range w.filters {
		r.receive(bytes)
	}

	return len(bytes), nil
}

func (w *worker) StartRouterFilter(router *Router) {
	w.mu.Lock()
	defer w.mu.Unlock()

	select {
	case <-w.stopper.C:
		return
	default:
		filter := newFilter(w, router)
		w.filters[router.id] = filter

		go func() {
			defer delete(w.filters, router.id)
			filter.start()
		}()
	}
}

// nolint:gosec //ignore this.
func (w *worker) start() {
	go func() {
		defer func() {
			w.stop()
			logger.Infof("worker [%s] stopped", w.id)
		}()

		if w.command == "" {
			<-w.stopper.C

			return
		}

		for {
			select {
			case <-w.stopper.C:
				return
			default:
				logger.Infof("worker [%s] command: %s", w.id, w.command)

				w.cmd = exec.Command("/bin/sh", "-c", w.command)

				setCmdSysProcAttr(w.cmd)

				w.cmd.Stdout = w
				w.cmd.Stderr = os.Stderr

				if err := w.cmd.Run(); err != nil {
					logger.Errorf("worker [%s] command error: %+v, command: %s", w.id, err, w.command)

					// if the command is generated dynamic, should not restart by self, send error instead.
					if w.dynamic {
						w.server.receiveWorkerError(err)

						return
					}

					select {
					case <-w.stopper.C:
						return
					default:
						logger.Errorf("worker [%s] failed, retry after 10s! command: %s", w.id, w.command)
						time.Sleep(CommandFailRetryInterval)
					}
				}

				// if the command is generated dynamic, should not restart by self, send error instead.
				if w.dynamic {
					w.server.receiveWorkerError(fmt.Errorf("%w: worker [%s]", ErrWorkerCommandStopped, w.id))

					return
				}
			}
		}
	}()
}

// stop will stop the current worker, but it may retry to start later.
// it will not close the Stopper.stop chan.
func (w *worker) stop() {
	defer func() {
		if err := recover(); err != nil {
			logger.Warnf("worker [%s] close error: %+v", w.id, err)
		}
	}()

	if w.cmd != nil {
		logger.Infof("worker [%s] command stopping: %s", w.id, w.command)

		if err := killCmd(w.cmd); err != nil {
			logger.Warnf("worker [%s] kill command error: %+v", w.id, err)
		}

		w.cmd = nil
	}

	w.stopFilters()
}

// shutdown will close the current worker, even may close the server,
// depending on the effect scope of the Stopper.
func (w *worker) shutdown() {
	// let server do the worker shutdown.
	w.server.shutdownWorker(w)
}

func (w *worker) stopFilters() {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, filter := range w.filters {
		filter.stop()
	}
}

func startWorker(s *Server, command string, dynamic bool) *worker {
	runWorker := newWorker(s, command, dynamic)

	if len(s.routers) > 0 {
		for _, r := range s.routers {
			runWorker.StartRouterFilter(r)
		}
	}

	runWorker.start()

	return runWorker
}

func newWorker(workerServer *Server, command string, dynamic bool) *worker {
	workerID := fmt.Sprintf("%s-%d", workerServer.id, len(workerServer.workers))
	if command == "" {
		workerID = fmt.Sprintf("%s-default", workerServer.id)
	}

	return &worker{
		mu:      sync.Mutex{},
		id:      workerID,
		server:  workerServer,
		stopper: workerServer.stopper,
		command: command,
		dynamic: dynamic,
		filters: make(map[string]*Filter, defaultMapSize),
	}
}
