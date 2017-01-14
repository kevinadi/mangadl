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

var mockmanga = Site{
	url:      "http://localhost:54321",
	img:      func(*goquery.Document) string { return "" },
	pageList: func(string, int, *goquery.Document) []string { return []string{""} }}

func TestHttpTest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	res, err := http.Get(ts.URL)
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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello, client")
	}))
	defer ts.Close()

	out := downloadImage(ts.URL)
	expect := []byte("Hello, client")

	if bytes.Compare(out, expect) != 0 {
		fmt.Println(out)
		fmt.Println(expect)
		t.Fail()
	}
}
