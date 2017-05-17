package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"strings"

	mp "github.com/mackerelio/go-mackerel-plugin-helper"
)

// https://github.com/memcached/memcached/blob/master/doc/protocol.txt
var graphdef map[string]mp.Graphs = map[string]mp.Graphs{
	"memcached.connections": {
		Label: "Memcached Connections",
		Unit:  "integer",
		Metrics: []mp.Metrics{
			{Name: "curr_connections", Label: "Connections", Diff: false},
		},
	},
	"memcached.cmd": {
		Label: "Memcached Command",
		Unit:  "integer",
		Metrics: []mp.Metrics{
			{Name: "cmd_get", Label: "Get", Diff: true, Type: "uint64"},
			{Name: "cmd_set", Label: "Set", Diff: true, Type: "uint64"},
			{Name: "cmd_flush", Label: "Flush", Diff: true, Type: "uint64"},
			{Name: "cmd_touch", Label: "Touch", Diff: true, Type: "uint64"},
		},
	},
	"memcached.hitmiss": {
		Label: "Memcached Hits/Misses",
		Unit:  "integer",
		Metrics: []mp.Metrics{
			{Name: "get_hits", Label: "Get Hits", Diff: true, Type: "uint64"},
			{Name: "get_misses", Label: "Get Misses", Diff: true, Type: "uint64"},
			{Name: "delete_hits", Label: "Delete Hits", Diff: true, Type: "uint64"},
			{Name: "delete_misses", Label: "Delete Misses", Diff: true, Type: "uint64"},
			{Name: "incr_hits", Label: "Incr Hits", Diff: true, Type: "uint64"},
			{Name: "incr_misses", Label: "Incr Misses", Diff: true, Type: "uint64"},
			{Name: "cas_hits", Label: "Cas Hits", Diff: true, Type: "uint64"},
			{Name: "cas_misses", Label: "Cas Misses", Diff: true, Type: "uint64"},
			{Name: "touch_hits", Label: "Touch Hits", Diff: true, Type: "uint64"},
			{Name: "touch_misses", Label: "Touch Misses", Diff: true, Type: "uint64"},
		},
	},
	"memcached.evictions": {
		Label: "Memcached Evictions",
		Unit:  "integer",
		Metrics: []mp.Metrics{
			{Name: "evictions", Label: "Evictions", Diff: true, Type: "uint64"},
		},
	},
	"memcached.unfetched": {
		Label: "Memcached Unfetched",
		Unit:  "integer",
		Metrics: []mp.Metrics{
			{Name: "expired_unfetched", Label: "Expired unfetched", Diff: true, Type: "uint64"},
			{Name: "evicted_unfetched", Label: "Evicted unfetched", Diff: true, Type: "uint64"},
		},
	},
	"memcached.rusage": {
		Label: "Memcached Resouce Usage",
		Unit:  "float",
		Metrics: []mp.Metrics{
			{Name: "rusage_user", Label: "User", Diff: true},
			{Name: "rusage_system", Label: "System", Diff: true},
		},
	},
	"memcached.bytes": {
		Label: "Memcached Traffics",
		Unit:  "bytes",
		Metrics: []mp.Metrics{
			{Name: "bytes_read", Label: "Read", Diff: true, Type: "uint64"},
			{Name: "bytes_written", Label: "Write", Diff: true, Type: "uint64"},
		},
	},
}

type MemcachedPlugin struct {
	Target   string
	Tempfile string
}

func (m MemcachedPlugin) FetchMetrics() (map[string]interface{}, error) {
	conn, err := net.Dial("tcp", m.Target)
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(conn, "stats")
	return m.ParseStats(conn)
}

func (m MemcachedPlugin) ParseStats(conn io.Reader) (map[string]interface{}, error) {
	scanner := bufio.NewScanner(conn)
	stat := make(map[string]interface{})

	for scanner.Scan() {
		line := scanner.Text()
		s := string(line)
		if s == "END" {
			return stat, nil
		}

		res := strings.Split(s, " ")
		if res[0] == "STAT" {
			stat[res[1]] = res[2]
		}
	}
	if err := scanner.Err(); err != nil {
		return stat, err
	}
	return nil, nil
}

func (m MemcachedPlugin) GraphDefinition() map[string]mp.Graphs {
	return graphdef
}

func main() {
	optHost := flag.String("host", "localhost", "Hostname")
	optPort := flag.String("port", "11211", "Port")
	optTempfile := flag.String("tempfile", "", "Temp file name")
	flag.Parse()

	var memcached MemcachedPlugin

	memcached.Target = fmt.Sprintf("%s:%s", *optHost, *optPort)
	helper := mp.NewMackerelPlugin(memcached)
	helper.Tempfile = *optTempfile

	helper.Run()
}
