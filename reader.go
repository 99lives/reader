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

var freeThreads = 16
var numStarted, numFinished, numErrors, numBytes int64 = 1, 0, 0, 0
var foundUrls = make(map[string]bool)
var chUrls = make(chan string, 10000)
var chToCrawl = make(chan string, 10000)
var chFinished = make(chan bool, 10000)

// Extract all http** links from a given webpage
func crawl(url string, ch chan string, chFinished chan bool) {
	//fmt.Println("\nCrawling:  " + url)

	defer func() {
		// Notify that we're done after this function

		select {
		case chFinished <- true:
		default:
			fmt.Println("chFinished would block")
		}
	}()

	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("error in get")
		return
	}

	output, err := httputil.DumpResponse(resp, true)

	if err != nil {
		fmt.Println("error in dumpresponse")
		return
	}
	if resp.Body == nil {
		fmt.Println("error in body")
		return
	}
	numBytes += int64(len(output))

	if err != nil {
		numErrors++
		fmt.Println("\nERROR:  " + err.Error())
		fmt.Println("\nERROR: Failed to crawl \"" + url + "\"")
		return
	}
	//fmt.Println("content length", resp.ContentLength)

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
			//dump, err := httputil.DumpResponse(resp, true)
			//if err != nil {
			//log.Fatal(err)
			//}

			//fmt.Println("response len:", string(dump))

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

				select {
				case ch <- url:
				default:
					fmt.Println("ch would block")
				}
			}
		}
	}
}
func stats() {
	var bytes [10]int64
	var index = 0
	var lastBytes int64 = 0

	for {
		bytes[index] = numBytes - lastBytes
		index++
		if index == len(bytes) {
			index = 0
		}
		lastBytes = numBytes
		var avg int64 = 0
		for i := range bytes {
			avg += bytes[i]
		}
		avg = avg / 10
		fmt.Println(" numStarted:", numStarted, "numFinished:", numFinished, "found:", len(foundUrls), "numErrors:", numErrors, numBytes, "KB/s", avg/1024)
		fmt.Println(" chToCrawl:", len(chToCrawl), " chUrls:", len(chUrls), " chFinished:", len(chFinished))

		//fmt.Println(index)
		//fmt.Println(bytes)
		time.Sleep(1 * time.Second)
	}
}
func main() {

	seedUrls := os.Args[1:]

	go stats()
	// Channels

	// Kick off the crawl process (concurrently)
	for _, url := range seedUrls {
		go crawl(url, chUrls, chFinished)
	}

	// Subscribe to both channels
	for {
		time.Sleep(1 * time.Millisecond)

		select {
		case url := <-chUrls:

			ok := foundUrls[url]
			if !ok {
				foundUrls[url] = true
				select {
				case chToCrawl <- url:
				default:
					fmt.Println("chToCrawl would block")
				}

			} else {
				//fmt.Println("\nAllready Added", url)
			}
		case <-chFinished:
			numFinished++
			freeThreads++
		default:
			if freeThreads > 0 {
				//fmt.Println("\nDefault")
				select {
				case url := <-chToCrawl:
					go crawl(url, chUrls, chFinished)
					numStarted++
					freeThreads--
				default:
					//fmt.Println("\no activity")
				}

			}
			//fmt.Println("\nFINISHED!")
		}

	}

	// We're done! Print the results...

	//fmt.Println("\nFound", len(foundUrls), "unique urls:\n")

	for url, _ := range foundUrls {
		fmt.Println(" - " + url)
	}

	close(chUrls)
}
