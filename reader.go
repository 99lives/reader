package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Helper function to pull the href attribute from a Token
func getHref(t html.Token) (ok bool, href string) {
	// Iterate over all of the Token's attributes until we find an "href"
	for _, a := range t.Attr {
		if a.Key == "href" {
			href = a.Val
			ok = true
		}
	}

	// "bare" return will return the variables (ok, href) as defined in
	// the function definition
	return
}

var numStarted, numFinished, numErrors, numBytes int64 = 1, 0, 0, 0
var foundUrls = make(map[string]bool)

// Extract all http** links from a given webpage
func crawl(url string, ch chan string, chFinished chan bool) {
	resp, err := http.Get(url)

	defer func() {
		// Notify that we're done after this function
		chFinished <- true
	}()

	if err != nil {
		numErrors++
		fmt.Println("\nERROR:  " + err.Error())
		fmt.Println("\nERROR: Failed to crawl \"" + url + "\"")
		return
	}
	fmt.Println("content length", resp.ContentLength)

	b := resp.Body
	defer b.Close() // close Body when the function returns
	/*
		dump, err := httputil.DumpResponse(resp, true)
		if err != nil {
			//log.Fatal(err)
		}

		fmt.Printf("response len:%d\n\n%q", len(dump))
	*/
	z := html.NewTokenizer(b)

	for {
		tt := z.Next()

		switch {
		case tt == html.ErrorToken:
			// End of the document, we're done
			dump, err := httputil.DumpResponse(resp, true)
			if err != nil {
				//log.Fatal(err)
			}

			fmt.Println("response len:", len(dump))
			numBytes += int64(len(dump))
			return
		case tt == html.StartTagToken:
			t := z.Token()

			// Check if the token is an <a> tag
			isAnchor := t.Data == "a"
			if !isAnchor {
				continue
			}

			// Extract the href value, if there is one
			ok, url := getHref(t)
			if !ok {
				continue
			}

			// Make sure the url begines in http**
			hasProto := strings.Index(url, "http") == 0
			if hasProto {
				ch <- url
			}
		}
	}
}
func stats() {
	for {
		fmt.Println("\n", numStarted, numFinished, len(foundUrls), numErrors, numBytes)
		time.Sleep(1 * time.Second)
	}
}
func main() {

	seedUrls := os.Args[1:]

	go stats()
	// Channels
	chUrls := make(chan string)
	chFinished := make(chan bool)

	// Kick off the crawl process (concurrently)
	for _, url := range seedUrls {
		go crawl(url, chUrls, chFinished)
	}

	// Subscribe to both channels
	for {
		time.Sleep(500 * time.Millisecond)

		select {
		case url := <-chUrls:

			ok := foundUrls[url]
			if !ok {
				fmt.Println("\nAdding", url)
				numStarted++
				go crawl(url, chUrls, chFinished)
				foundUrls[url] = true
			} else {
				fmt.Println("\nAllready Added", url)
			}
		case <-chFinished:
			numFinished++
			fmt.Println("\nFINISHED!")
		}

	}

	// We're done! Print the results...

	fmt.Println("\nFound", len(foundUrls), "unique urls:\n")

	for url, _ := range foundUrls {
		fmt.Println(" - " + url)
	}

	close(chUrls)
}
