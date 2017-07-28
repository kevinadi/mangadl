package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

/* manga containing 3 pages */
var pageHTML = `<html>
<body>
<select id="pageList">
<option value="/page1">1</option>
<option value="/page2">2</option>
<option value="/page3">3</option>
</select>
<img src="%s" id="image">
<select id="pageList">
<option value="/page1">1</option>
<option value="/page2">2</option>
<option value="/page3">3</option>
</select>
</body>
</html>
`

var imageRGBA = image.NewRGBA(image.Rect(0, 0, 10, 10))
var imageBuffer = new(bytes.Buffer)
var imageJPG = jpeg.Encode(imageBuffer, imageRGBA, nil)

var tsImage = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, imageBuffer)
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
		doc.Find("select#pageList").First().Find("option").Each(func(i int, s *goquery.Selection) {
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

func zipReader(b []byte) []DownloadResult {
	var output []DownloadResult
	reader := bytes.NewReader(b)
	archiveReader, _ := zip.NewReader(reader, int64(len(b)))
	for _, f := range archiveReader.File {
		resName := f.Name
		rc, _ := f.Open()
		resContents, _ := ioutil.ReadAll(rc)
		output = append(output, DownloadResult{
			Name:    resName,
			Content: resContents})
	}
	return output
}

func TestHttptestServers(t *testing.T) {
	/* test that the image httptest returns "Image" */
	res, _ := http.Get(tsImage.URL)
	resImg, _ := ioutil.ReadAll(res.Body)
	expectImg := imageBuffer.Bytes()
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

	expect := imageBuffer.Bytes()
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

	expectImage := imageBuffer.Bytes()
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
		Content: imageBuffer.Bytes()}
	if !reflect.DeepEqual(got, expect) {
		fmt.Printf("Got: %s\n", got)
		fmt.Printf("Expect: %s\n", expect)
		t.Fail()
	}

	got = <-result
	expect = DownloadResult{
		Name:    "image-001-002.jpg",
		Content: imageBuffer.Bytes()}
	if !reflect.DeepEqual(got, expect) {
		fmt.Printf("Got: %s\n", got)
		fmt.Printf("Expect: %s\n", expect)
		t.Fail()
	}

	got = <-result
	expect = DownloadResult{
		Name:    "image-002-001.jpg",
		Content: imageBuffer.Bytes()}
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
				Content: imageBuffer.Bytes()})
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

func TestCbzChan(t *testing.T) {
	/* test using buffer instead of file */
	var buf bytes.Buffer

	/* expected result */
	expect := []DownloadResult{
		DownloadResult{
			Name:    "image-001-001.jpg",
			Content: imageBuffer.Bytes()},
		DownloadResult{
			Name:    "image-001-002.jpg",
			Content: imageBuffer.Bytes()},
		DownloadResult{
			Name:    "image-002-001.jpg",
			Content: imageBuffer.Bytes()}}

	/* create, fill, and close results channel */
	downloadedPages := make(chan DownloadResult, 3)
	for _, res := range expect {
		downloadedPages <- res
	}
	close(downloadedPages)

	/* TEST */
	var wg sync.WaitGroup
	wg.Add(1)
	go cbzChan(&buf, downloadedPages, &wg)
	wg.Wait()

	/* read the result zip archive */
	got := zipReader(buf.Bytes())

	/* got == expect ? */
	if !reflect.DeepEqual(expect, got) {
		fmt.Printf("Got: %s\n", got)
		fmt.Printf("Expect: %s\n", expect)
		t.Fail()
	}
}

func TestComicextra(t *testing.T) {
	t.Run("Image", func(t *testing.T) {
		/* Image URL */
		html := `
		<img id="main_img" class="chapter_img" src="http://2.bp.blogspot.com/g4M04SEdkwl1iGNHuRIq2PvqIdTIKuX5sjGPgVaQQmOJXu793uilskOe6cABXqKfAwy1wi4g-qzE=s0" data-width="820" alt="Valerian and Laureline 1 Page 1" style="width: 100%;">
		`
		htmlDocument, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
		expect := "http://2.bp.blogspot.com/g4M04SEdkwl1iGNHuRIq2PvqIdTIKuX5sjGPgVaQQmOJXu793uilskOe6cABXqKfAwy1wi4g-qzE=s0"
		got := comicextra.img(htmlDocument)
		if !reflect.DeepEqual(expect, got) {
			fmt.Printf("Got: %s\n", got)
			fmt.Printf("Expect: %s\n", expect)
			t.Fail()
		}
	})

	t.Run("PageList", func(t *testing.T) {
		/* Page list */
		html := `
		<select name="page_select" class="full-select"><option selected="selected">1 </option></select>
		<select name="page_select" class="full-select"><option value="http://www.comicextra.com/valerian-and-laureline/chapter-1" selected="selected">1 </option><option value="http://www.comicextra.com/valerian-and-laureline/chapter-1/2">2 </option><option value="http://www.comicextra.com/valerian-and-laureline/chapter-1/3">3 </option></select>
		`
		htmlDocument, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
		expectArray := []string{
			"http://www.comicextra.com/valerian-and-laureline/chapter-1",
			"http://www.comicextra.com/valerian-and-laureline/chapter-1/2",
			"http://www.comicextra.com/valerian-and-laureline/chapter-1/3",
		}
		gotArray := comicextra.pageList("manga", 1, htmlDocument)
		if !reflect.DeepEqual(expectArray, gotArray) {
			fmt.Printf("Got: %s\n", gotArray)
			fmt.Printf("Expect: %s\n", expectArray)
			t.Fail()
		}
	})

	t.Run("URL", func(t *testing.T) {
		manga := "valerian-and-laureline"
		got := comicextra.url + manga + comicextra.chapter(1) + comicextra.page("1")
		expect := "http://www.comicextra.com/valerian-and-laureline/chapter-1/1"
		if !reflect.DeepEqual(expect, got) {
			fmt.Printf("Got: %s\n", got)
			fmt.Printf("Expect: %s\n", expect)
			t.Fail()
		}
	})
}

func TestMangareader(t *testing.T) {
	t.Run("Image", func(t *testing.T) {
		/* Image URL */
		html := `
		<img id="img" width="800" height="1263" src="http://i10.mangareader.net/naruto/1/naruto-1564773.jpg" alt="Naruto 1 - Page 1" name="img">
		`
		htmlDocument, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
		expect := "http://i10.mangareader.net/naruto/1/naruto-1564773.jpg"
		got := mangareader.img(htmlDocument)
		if !reflect.DeepEqual(expect, got) {
			fmt.Printf("Got: %s\n", got)
			fmt.Printf("Expect: %s\n", expect)
			t.Fail()
		}
	})

	t.Run("PageList", func(t *testing.T) {
		/* Page list */
		html := `
		<select id="pageMenu" name="pageMenu"><option value="/naruto/1" selected="selected">1</option>
		<option value="/naruto/1/2">2</option>
		<option value="/naruto/1/3">3</option>
		</select>
		<select id="pageMenu" name="pageMenu"><option value="/naruto/1" selected="selected">1</option>
		<option value="/naruto/1/2">2</option>
		<option value="/naruto/1/3">3</option>
		</select>
		`
		htmlDocument, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
		expect := []string{
			"http://www.mangareader.net/naruto/1",
			"http://www.mangareader.net/naruto/1/2",
			"http://www.mangareader.net/naruto/1/3",
		}
		got := mangareader.pageList("manga", 1, htmlDocument)
		if !reflect.DeepEqual(expect, got) {
			fmt.Printf("Got: %s\n", got)
			fmt.Printf("Expect: %s\n", expect)
			t.Fail()
		}
	})

	t.Run("URL", func(t *testing.T) {
		manga := "naruto"
		got := mangareader.url + manga + mangareader.chapter(1) + mangareader.page("1")
		expect := "http://www.mangareader.net/naruto/1/1"
		if !reflect.DeepEqual(expect, got) {
			fmt.Printf("Got: %s\n", got)
			fmt.Printf("Expect: %s\n", expect)
			t.Fail()
		}
	})
}

func TestMangafox(t *testing.T) {
	t.Run("Image", func(t *testing.T) {
		/* Image link */
		html := `
		<img src="http://l.mfcdn.net/store/manga/8/01-001.0/compressed/naruto_v01_ch001_005.jpg?token=b0a60425c24cdb15e3a0d5681cd41b188d0d8a59&amp;ttl=1501300800" width="671" id="image" alt="Naruto 1: Uzumaki Naruto at MangaFox.me">
		`
		htmlDocument, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
		expect := "http://l.mfcdn.net/store/manga/8/01-001.0/compressed/naruto_v01_ch001_005.jpg?token=b0a60425c24cdb15e3a0d5681cd41b188d0d8a59&ttl=1501300800"
		got := mangafox.img(htmlDocument)
		if !reflect.DeepEqual(expect, got) {
			fmt.Printf("Got: %s\n", got)
			fmt.Printf("Expect: %s\n", expect)
			t.Fail()
		}
	})

	t.Run("PageList", func(t *testing.T) {
		/* Page list */
		html := `
		<select onchange="change_page(this)" class="m">
		<option value="1" selected="selected">1</option><option value="2">2</option><option value="3">3</option><option value="0">Comments</option>
		</select>
		<select onchange="change_page(this)" class="m">
		<option value="1" selected="selected">1</option><option value="2">2</option><option value="3">3</option><option value="0">Comments</option>
		</select>
		`
		htmlDocument, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
		expect := []string{
			"http://mangafox.me/manga/naruto/c001/1.html",
			"http://mangafox.me/manga/naruto/c001/2.html",
			"http://mangafox.me/manga/naruto/c001/3.html",
		}
		got := mangafox.pageList("naruto", 1, htmlDocument)
		if !reflect.DeepEqual(expect, got) {
			fmt.Printf("Got: %s\n", got)
			fmt.Printf("Expect: %s\n", expect)
			t.Fail()
		}
	})

	t.Run("URL", func(t *testing.T) {
		manga := "naruto"
		got := mangafox.url + manga + mangafox.chapter(1) + mangafox.page("1")
		expect := "http://mangafox.me/manga/naruto/c001/1.html"
		if !reflect.DeepEqual(expect, got) {
			fmt.Printf("Got: %s\n", got)
			fmt.Printf("Expect: %s\n", expect)
			t.Fail()
		}
	})
}
