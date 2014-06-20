package collectors

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
)

type ProgramCollector struct {
	Path     string
	Interval time.Duration
}

func InitPrograms(cpath string) {
	cdir, err := os.Open(cpath)
	if err != nil {
		slog.Infoln(err)
		return
	}
	idirs, err := cdir.Readdir(0)
	if err != nil {
		slog.Infoln(err)
		return
	}
	for _, idir := range idirs {
		i, err := strconv.Atoi(idir.Name())
		if err != nil {
			slog.Infoln(err)
			continue
		}
		interval := time.Second * time.Duration(i)
		dir, err := os.Open(filepath.Join(cdir.Name(), idir.Name()))
		if err != nil {
			slog.Infoln(err)
			continue
		}
		files, err := dir.Readdir(0)
		if err != nil {
			slog.Infoln(err)
			continue
		}
		for _, file := range files {
			if file.Mode()&0111 == 0 {
				continue
			}
			collectors = append(collectors, &ProgramCollector{
				Path:     filepath.Join(dir.Name(), file.Name()),
				Interval: interval,
			})
		}
	}
}

func (c *ProgramCollector) Run(dpchan chan<- *opentsdb.DataPoint) {
	if c.Interval == 0 {
		for {
			next := time.After(DefaultFreq)
			if err := c.runProgram(dpchan); err != nil {
				slog.Infoln(err)
			}
			<-next
			slog.Infoln("restarting", c.Path)
		}
	} else {
		for {
			next := time.After(c.Interval)
			c.runProgram(dpchan)
			<-next
		}
	}
}

func (c *ProgramCollector) Init() {
}

func (c *ProgramCollector) runProgram(dpchan chan<- *opentsdb.DataPoint) (progError error) {
	cmd := exec.Command(c.Path)
	pr, pw := io.Pipe()
	s := bufio.NewScanner(pr)
	cmd.Stdout = pw
	err := cmd.Start()
	if err != nil {
		return err
	}
	go func() {
		progError = cmd.Wait()
		pw.Close()
	}()
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		sp := strings.Fields(line)
		if len(sp) < 3 {
			continue
		}
		ts, err := strconv.ParseInt(sp[1], 10, 64)
		if err != nil {
			continue
		}
		dp := opentsdb.DataPoint{
			Metric:    sp[0],
			Timestamp: ts,
			Value:     sp[2],
			Tags:      opentsdb.TagSet{"host": host},
		}
		for _, tag := range sp[3:] {
			tsp := strings.Split(tag, "=")
			if len(tsp) != 2 {
				slog.Fatal("bad tag", tsp)
				continue
			}
			dp.Tags[tsp[0]] = tsp[1]
		}
		dpchan <- &dp
	}
	if err := s.Err(); err != nil {
		return err
	}
	return
}

func (c *ProgramCollector) Name() string {
	return c.Path
}
