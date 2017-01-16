package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"reflect"

	"sync"

	"io/ioutil"

	"github.com/PuerkitoBio/goquery"
)

/* manga containing 3 pages */
var pageHTML = `<html>
<body>
<option value="/page1">1</option>
<option value="/page2">2</option>
<option value="/page3">3</option>
<img src="%s" id="image">
</body>
</html>
`

var tsImage = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Image")
}))

var tsPage = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, pageHTML, tsImage.URL)
}))

var mockmanga = Site{
	url: tsPage.URL + "/",
	img: func(doc *goquery.Document) string {
		imageURL, _ := doc.Find("#image").Attr("src")
		return imageURL
	},
	pageList: func(manga string, chapter int, doc *goquery.Document) []string {
		var links []string
		doc.Find("option").Each(func(i int, s *goquery.Selection) {
			link, _ := s.Attr("value")
			formattedLink := fmt.Sprintf("%s://%s%s", doc.Url.Scheme, doc.Url.Host, link)
			links = append(links, formattedLink)
		})
		return links
	},
	page:        func(n string) string { return fmt.Sprintf("/%s", n) },
	chapter:     func(n int) string { return fmt.Sprintf("/%d", n) },
	parChapters: 1,
	parPages:    1}

func TestHttptestServers(t *testing.T) {
	/* test that the image httptest returns "Image" */
	res, _ := http.Get(tsImage.URL)
	resImg, _ := ioutil.ReadAll(res.Body)
	expectImg := []byte("Image")
	if !reflect.DeepEqual(resImg, expectImg) {
		fmt.Printf("URL: %s\n", tsImage.URL)
		fmt.Printf("Got: %s\n", resImg)
		fmt.Printf("Expect: %s\n", expectImg)
		t.Fail()
	}

	/* test that the page httptest returns the correct page with image link */
	res, _ = http.Get(tsPage.URL)
	resPageBytes, _ := ioutil.ReadAll(res.Body)
	resPage := fmt.Sprintf("%s", resPageBytes)
	expectPage := fmt.Sprintf(pageHTML, tsImage.URL)
	if !reflect.DeepEqual(resPage, expectPage) {
		fmt.Printf("URL: %s\n", tsPage.URL)
		fmt.Printf("Got: %s\n", resPage)
		fmt.Printf("Expect: %s\n", expectPage)
		t.Fail()
	}
}

func TestMockPage(t *testing.T) {
	resp, _ := http.Get(tsPage.URL)
	doc, _ := goquery.NewDocumentFromResponse(resp)

	img := mockmanga.img(doc)
	expectImg := tsImage.URL
	if !reflect.DeepEqual(img, expectImg) {
		fmt.Printf("Got: %s\n", img)
		fmt.Printf("Expect: %s\n", expectImg)
		t.Fail()
	}

	pageList := mockmanga.pageList("", 1, doc)
	expectLinks := []string{
		tsPage.URL + "/page1",
		tsPage.URL + "/page2",
		tsPage.URL + "/page3"}
	if !reflect.DeepEqual(pageList, expectLinks) {
		fmt.Printf("Got: %s\n", pageList)
		fmt.Printf("Expect: %s\n", expectLinks)
		t.Fail()
	}
}

func TestDownloadImage(t *testing.T) {
	got := downloadImage(tsImage.URL)

	expect := []byte("Image")
	if !reflect.DeepEqual(got, expect) {
		fmt.Printf("Image src: %s\n", tsImage.URL)
		fmt.Printf("Got: %s\n", got)
		fmt.Printf("Expect: %s\n", expect)
		t.Fail()
	}
}

func TestGetFirstPage(t *testing.T) {
	sites["mockmanga"] = mockmanga

	links, pageImageBytes := getFirstPage("mockmanga", "manga-name", 1)

	expectLinks := []string{
		tsPage.URL + "/page1",
		tsPage.URL + "/page2",
		tsPage.URL + "/page3"}
	if !reflect.DeepEqual(links, expectLinks) {
		fmt.Printf("Got: %s\n", links)
		fmt.Printf("Expect: %s\n", expectLinks)
		t.Fail()
	}

	expectImage := []byte("Image")
	if !reflect.DeepEqual(pageImageBytes, expectImage) {
		fmt.Printf("Got: %s\n", pageImageBytes)
		fmt.Printf("Expect: %s\n", expectImage)
		t.Fail()
	}
}

func TestDownloadPage(t *testing.T) {
	sites["mockmanga"] = mockmanga
	numJobs := 3

	dljob := make(chan DownloadJob, numJobs)
	dljob <- DownloadJob{
		Chapter: 1,
		Page:    1,
		Link:    tsPage.URL}
	dljob <- DownloadJob{
		Chapter: 1,
		Page:    2,
		Link:    tsPage.URL}
	dljob <- DownloadJob{
		Chapter: 2,
		Page:    1,
		Link:    tsPage.URL}
	close(dljob)
	result := make(chan DownloadResult, numJobs)
	var wg sync.WaitGroup
	wg.Add(numJobs)

	go downloadPage(1, "mockmanga", dljob, result, &wg)
	wg.Wait()

	got := <-result
	expect := DownloadResult{
		Name:    "image-001-001.jpg",
		Content: []byte("Image")}
	if !reflect.DeepEqual(got, expect) {
		fmt.Printf("Got: %s\n", got)
		fmt.Printf("Expect: %s\n", expect)
		t.Fail()
	}

	got = <-result
	expect = DownloadResult{
		Name:    "image-001-002.jpg",
		Content: []byte("Image")}
	if !reflect.DeepEqual(got, expect) {
		fmt.Printf("Got: %s\n", got)
		fmt.Printf("Expect: %s\n", expect)
		t.Fail()
	}

	got = <-result
	expect = DownloadResult{
		Name:    "image-002-001.jpg",
		Content: []byte("Image")}
	if !reflect.DeepEqual(got, expect) {
		fmt.Printf("Got: %s\n", got)
		fmt.Printf("Expect: %s\n", expect)
		t.Fail()
	}
}

func TestDownloadChapter(t *testing.T) {
	sites["mockmanga"] = mockmanga
	numChapters := 3

	chapters := make(chan int, numChapters)
	downloadedPages := make(chan DownloadResult, 3*numChapters)
	var wg sync.WaitGroup

	for i := 1; i <= numChapters; i++ {
		chapters <- i
	}
	close(chapters)

	wg.Add(numChapters)
	go downloadChapter("mockmanga", "manga_test", chapters, downloadedPages, 1, &wg)
	wg.Wait()
	close(downloadedPages)

	var expect []DownloadResult
	for c := 1; c <= 3; c++ {
		for p := 0; p <= 2; p++ {
			expect = append(expect, DownloadResult{
				Name:    fmt.Sprintf("image-%03d-%03d.jpg", c, p),
				Content: []byte("Image")})
		}
	}

	var got []DownloadResult
	for p := range downloadedPages {
		got = append(got, p)
	}

	if !reflect.DeepEqual(expect, got) {
		fmt.Printf("Expect: %s\n", expect)
		fmt.Printf("Got: %s\n", got)
		t.Fail()
	}
}
