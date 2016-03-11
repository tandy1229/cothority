// Monitor package handle the logging, collection and computation of
// statistical data. Every application can send some Measure (for the moment,
// we mostly measure the CPU time but it can be applied later for any kind of
// measures). The Monitor receives them and updates a Stats struct. This Stats
// struct can hold many different kinds of Measurements (the measure of a
// specific action such as "round time" or "verify time" etc). These
// measurements contain Values which compute the actual min/max/dev/avg values.
//
// The Proxy allows to relay Measure from
// clients to the listening Monitor. A starter feature is also the DataFilter
// which can apply some filtering rules to the data before making any
// statistics about them.
package monitor

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/dedis/cothority/lib/dbg"
)

// This file handles the collection of measurements, aggregates them and
// write CSV file reports

// listen is the address where to listen for the monitor. The endpoint can be a
// monitor.Proxy or a direct connection with measure.go
var Sink = "0.0.0.0"
var DefaultSinkPort = 10000

// Monitor struct is used to collect measures and make the statistics about
// them. It takes a stats object so it update that in a concurrent-safe manner
// for each new measure it receives.
type Monitor struct {
	listener     net.Listener
	listenerLock *sync.Mutex

	// Current conections
	conns map[string]net.Conn
	// and the mutex to play with it
	mutexConn sync.Mutex

	// Current stats
	stats *Stats
	// and the mutex to play with it
	mutexStats sync.Mutex

	// channel to give new measures
	measures chan *SingleMeasure

	// channel to notify the end of a connection
	// send the name of the connection when finishd
	done chan string
}

// NewMonitor returns a new monitor given the stats
func NewMonitor(stats *Stats) *Monitor {
	return &Monitor{
		conns:        make(map[string]net.Conn),
		stats:        stats,
		mutexStats:   sync.Mutex{},
		measures:     make(chan *SingleMeasure),
		done:         make(chan string),
		listenerLock: new(sync.Mutex),
	}
}

// Monitor will start listening for incoming connections on this address
// It needs the stats struct pointer to update when measures come
// Return an error if something went wrong during the connection setup
func (m *Monitor) Listen() error {
	ln, err := net.Listen("tcp", Sink+":"+strconv.Itoa(DefaultSinkPort))
	if err != nil {
		return fmt.Errorf("Error while monitor is binding address: %v", err)
	}
	m.listener = ln
	dbg.Lvl2("Monitor listening for stats on", Sink, ":", DefaultSinkPort)
	finished := false
	go func() {
		for {
			if finished {
				break
			}
			conn, err := ln.Accept()
			if err != nil {
				operr, ok := err.(*net.OpError)
				// We cant accept anymore we closed the listener
				if ok && operr.Op == "accept" {
					break
				}
				dbg.Lvl2("Error while monitor accept connection:", operr)
				continue
			}
			dbg.Lvl3("Monitor: new connection from", conn.RemoteAddr().String())
			m.mutexConn.Lock()
			m.conns[conn.RemoteAddr().String()] = conn
			go m.handleConnection(conn)
			m.mutexConn.Unlock()
		}
	}()
	for !finished {
		select {
		// new stats
		case measure := <-m.measures:
			m.update(measure)
		// end of a peer conn
		case peer := <-m.done:
			dbg.Lvl3("Connections left:", len(m.conns))
			m.mutexConn.Lock()
			delete(m.conns, peer)
			m.mutexConn.Unlock()
			// end of monitoring,
			if len(m.conns) == 0 {
				m.listenerLock.Lock()
				m.listener.Close()
				m.listener = nil
				finished = true
				m.listenerLock.Unlock()
				break
			}
		}
	}
	dbg.Lvl2("Monitor finished waiting")
	m.conns = make(map[string]net.Conn)
	return nil
}

// StopMonitor will close every connections it has
// And will stop updating the stats
func (m *Monitor) Stop() {
	dbg.Lvl2("Monitor Stop")
	m.listenerLock.Lock()
	if m.listener != nil {
		m.listener.Close()
	}
	m.listenerLock.Unlock()
	m.mutexConn.Lock()
	for _, c := range m.conns {
		c.Close()
	}
	m.mutexConn.Unlock()

}

// handleConnection will decode the data received and aggregates it into its
// stats
func (m *Monitor) handleConnection(conn net.Conn) {
	dec := json.NewDecoder(conn)
	nerr := 0
	for {
		measure := &SingleMeasure{}
		if err := dec.Decode(measure); err != nil {
			// if end of connection
			if err == io.EOF || strings.Contains(err.Error(), "closed") {
				break
			}
			// otherwise log it
			dbg.Lvl2("Error: monitor decoding from", conn.RemoteAddr().String(), ":", err)
			nerr += 1
			if nerr > 1 {
				dbg.Lvl2("Monitor: too many errors from", conn.RemoteAddr().String(), ": Abort.")
				break
			}
		}

		dbg.Lvlf3("Monitor: received a Measure from %s: %+v", conn.RemoteAddr().String(), measure)
		// Special case where the measurement is indicating a FINISHED step
		switch strings.ToLower(measure.Name) {
		case "end":
			dbg.Lvl3("Finishing monitor")
			m.done <- conn.RemoteAddr().String()
		default:
			m.measures <- measure
		}
	}
}

// updateMeasures will add that specific measure to the global stats
// in a concurrently safe manner
func (m *Monitor) update(meas *SingleMeasure) {
	m.mutexStats.Lock()
	// updating
	m.stats.Update(meas)
	m.mutexStats.Unlock()
}

// Stats returns the updated stats in a concurrent-safe manner
func (m *Monitor) Stats() *Stats {
	m.mutexStats.Lock()
	s := m.stats
	m.mutexStats.Unlock()
	return s
}
