package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"reflect"

	"sync"

	"github.com/PuerkitoBio/goquery"
)

var tsImage = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Image")
}))

var tsPage = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<html><body><img src='%s' id='image'></body></html>", tsImage.URL)
}))

var mockmanga = Site{
	url:         tsPage.URL + "/",
	img:         func(*goquery.Document) string { return tsImage.URL },
	pageList:    func(string, int, *goquery.Document) []string { return []string{"link1", "link2", "link3"} },
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
	expect := []byte("Image")

	if bytes.Compare(got, expect) != 0 {
		fmt.Printf("Got: %s\n", got)
		fmt.Printf("Expect: %s\n", expect)
		t.Fail()
	}
}

func TestGetFirstPage(t *testing.T) {
	sites["mockmanga"] = mockmanga

	links, pageImageBytes := getFirstPage("mockmanga", "manga-name", 1)

	expectLinks := []string{"link1", "link2", "link3"}
	if !reflect.DeepEqual(links, expectLinks) {
		fmt.Printf("Got: %s\n", links)
		fmt.Printf("Expect: %s\n", expectLinks)
		t.Fail()
	}

	expectImage := []byte("Image")
	if bytes.Compare(pageImageBytes, expectImage) != 0 {
		fmt.Printf("Got: %s\n", pageImageBytes)
		fmt.Printf("Expect: %s\n", expectImage)
		t.Fail()
	}
}

func TestDownloadPage(t *testing.T) {
	sites["mockmanga"] = mockmanga

	dljob := make(chan DownloadJob, 1)
	dljob <- DownloadJob{
		Chapter: 1,
		Page:    1,
		Link:    tsPage.URL}
	close(dljob)
	result := make(chan DownloadResult, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	downloadPage(1, "mockmanga", dljob, result, &wg)
	// wg.Wait()
	got := <-result
	expect := DownloadResult{
		Name:    "image-001-001.jpg",
		Content: []byte("Image")}
	if !reflect.DeepEqual(got, expect) {
		fmt.Printf("Got: %s\n", got)
		fmt.Printf("Expect: %s\n", expect)
		t.Fail()
	}
}
