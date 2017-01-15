package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

var tsImage = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello, client")
}))

var tsPage = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<html><body><a href='%s'></body></html>", tsImage.URL)
}))

var mockmanga = Site{
	url:         tsPage.URL + "/",
	img:         func(*goquery.Document) string { return "" },
	pageList:    func(string, int, *goquery.Document) []string { return []string{""} },
	page:        func(n string) string { return fmt.Sprintf("/%s", n) },
	chapter:     func(n int) string { return fmt.Sprintf("/%d", n) },
	parChapters: 1,
	parPages:    1}

func TestHttpTest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	getURL := ts.URL + "/blah/blah_blah"
	fmt.Println(getURL)
	res, err := http.Get(getURL)
	if err != nil {
		log.Fatal(err)
	}

	greeting, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s", greeting)
}

func TestDownloadImage(t *testing.T) {
	got := downloadImage(tsImage.URL)
	expect := []byte("Hello, client")

	if bytes.Compare(got, expect) != 0 {
		fmt.Printf("Got: %s\n", got)
		fmt.Printf("Expect: %s\n", expect)
		t.Fail()
	}
}

func TestGetFirstPage(t *testing.T) {
	sites["mockmanga"] = mockmanga

	links, pageImageBytes := getFirstPage("mockmanga", "manga-name", 1)
	fmt.Println(links)
	fmt.Println(pageImageBytes)
}
