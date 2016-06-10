package main

import (
	"archive/tar"
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	wg        sync.WaitGroup
	keyC      = make(chan string, 100)
	saveC     = make(chan ticket)
	done      = make(chan struct{})
	done1     = make(chan struct{})
	lost_keys = []string{""}
)

type ticket struct {
	key  string
	json []byte
}

func main() {
	go save()
	go func() {
		for {
			select {
			case key := <-keyC:
				fetch(key)
			case <-done:
				return
			}
		}
	}()

	for _, key := range lost_keys {
		wg.Add(1)
		keyC <- key
	}

	wg.Wait()
	close(done)
	<-done1
}

func fetch(key string) {
	resp, err := http.Get(key)
	if err != nil {
		log.Printf("Error: %s", err)
		wg.Done()
		return
	}

	if resp.StatusCode == 404 {
		log.Printf("not found: %s", key)
		wg.Done()
		return
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("error for %s: %s", key, err)
	}

	saveC <- ticket{key, b}

	resp.Body.Close()
	wg.Done()
}

func save() {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
Loop:
	for {
		select {
		case t := <-saveC:
			p := strings.Split(t.key, "/")
			hdr := &tar.Header{
				Name:    p[len(p)-1],
				Mode:    0600,
				Size:    int64(len(t.json)),
				ModTime: time.Now(),
			}
			if err := tw.WriteHeader(hdr); err != nil {
				log.Fatalln(err)
			}
			if _, err := tw.Write([]byte(t.json)); err != nil {
				log.Fatalln(err)
			}
			err := ioutil.WriteFile("/tmp/tickets.tar", buf.Bytes(), 0644)
			if err != nil {
				log.Fatalln(err)
			}

		case <-done:
			break Loop
		}
	}
	if err := tw.Close(); err != nil {
		log.Fatalln(err)
	}
	err := ioutil.WriteFile("/tmp/tickets.tar", buf.Bytes(), 0644)

	if err != nil {
		log.Fatalln(err)
	}
	done1 <- struct{}{}
}
