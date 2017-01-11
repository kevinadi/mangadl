package main

import (
	"archive/zip"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"

	"time"

	"github.com/PuerkitoBio/goquery"
)

// Site definition ...
type Site struct {
	url         string
	img         func(*goquery.Document) string
	pageList    func(string, int, *goquery.Document) <-chan string
	page        func(string) string
	chapter     func(int) string
	parChapters int
	parPages    int
}

var mangareader = Site{
	url: "http://www.mangareader.net/",
	img: func(doc *goquery.Document) string {
		/* <img id="img" src=[URL] name="img"> */
		imageURL, found := doc.Find("#img").Attr("src")
		if !found {
			log.Fatal("image not found: ", doc.Url.String())
		}
		return imageURL
	},
	pageList: func(manga string, chapter int, doc *goquery.Document) <-chan string {
		/* <option value=[pagelist]> */
		links := make(chan string)
		go func() {
			doc.Find("option").Each(func(i int, s *goquery.Selection) {
				link, _ := s.Attr("value")
				links <- link[1:] // link contains leading slash
			})
			close(links)
		}()
		return links
	},
	page:        func(n string) string { return fmt.Sprintf("/%s", n) },
	chapter:     func(n int) string { return fmt.Sprintf("/%d", n) },
	parChapters: 6,
	parPages:    6}

var mangafox = Site{
	url: "http://mangafox.me/manga/",
	img: func(doc *goquery.Document) string {
		imageURL, found := doc.Find("#image").Attr("src")
		if !found {
			fmt.Println(doc.Html())
			log.Fatal("image not found: ", doc.Url.String())
		}
		return imageURL
	},
	pageList: func(manga string, chapter int, doc *goquery.Document) <-chan string {
		chp := func(n int) string { return fmt.Sprintf("/c%03d", n) }
		page := func(n string) string { return fmt.Sprintf("/%s.html", n) }
		links := make(chan string)
		go func() {
			doc.Find("option").EachWithBreak(func(i int, s *goquery.Selection) bool {
				val, _ := s.Attr("value")
				if val == "0" {
					return false
				}
				link := manga + chp(chapter) + page(val)
				links <- link
				return true
			})
			close(links)
		}()
		return links
	},
	page:        func(n string) string { return fmt.Sprintf("/%s.html", n) },
	chapter:     func(n int) string { return fmt.Sprintf("/c%03d", n) },
	parChapters: 1,
	parPages:    1}

var sites = map[string]Site{
	"mangareader": mangareader,
	"mangafox":    mangafox}

// DownloadJob ...
type DownloadJob struct {
	Chapter int
	Page    int
	Link    string
}

// DownloadResult ...
type DownloadResult struct {
	Name    string
	Content []byte
}

func downloadImage(url string) []byte {
	for retry := 1; retry <= 3; retry++ {
		/* open url */
		response, e := http.Get(url)
		if e != nil {
			log.Println("Error getting", url, "retrying...", retry)
			time.Sleep(time.Second)
			continue
		}
		defer response.Body.Close()

		/* download image data ([]byte) from url */
		data, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Fatalf("ioutil.ReadAll -> %v", err)
		}
		return data
	}
	log.Fatal("Error downloading image after 3 retries:", url)
	return nil
}

func createCBZchan(cbzName string, downloadedPages <-chan DownloadResult, wgCBZ *sync.WaitGroup) {
	/* create the zip file */
	buf, createErr := os.Create(cbzName)
	if createErr != nil {
		log.Fatal(createErr)
	}
	zipWriter := zip.NewWriter(buf)
	log.Println("Creating cbz:", cbzName)

	/* write to zipfile as each finished page arrives in channel */
	for file := range downloadedPages {
		/* create zip writer with header of filename, DEFLATE method, and current time */
		header := zip.FileHeader{
			Name:   file.Name,
			Method: zip.Deflate}
		header.SetModTime(time.Now())
		f, err := zipWriter.CreateHeader(&header)
		if err != nil {
			log.Fatal(err)
		}

		/* write content to zip archive */
		_, err = f.Write(file.Content)
		if err != nil {
			log.Fatal(err)
		}
	}

	/* close the archive */
	err := zipWriter.Close()
	if err != nil {
		log.Fatal(err)
	}
	log.Println(cbzName, "closed")

	/* signal to downloadChapters that all writes are done and the archive is closed */
	wgCBZ.Done()
}

func getFirstPage(site, manga, baseurl string, chapter int) (<-chan string, []byte) {
	/* get first page html */
	url := sites[site].url + manga + sites[site].chapter(chapter) + sites[site].page("1")

	resp, errget := http.Get(url)
	if errget != nil {
		log.Fatal("Error getting ", url)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		log.Println(doc.Html())
		log.Fatal(err)
	}

	/* get first page image */
	pageImageURL := sites[site].img(doc)
	pageImageBytes := downloadImage(pageImageURL)

	/* get all pages links */
	links := sites[site].pageList(manga, chapter, doc)

	return links, pageImageBytes
}

func downloadPage(n int, site string, jobs <-chan DownloadJob, downloadedPages chan<- DownloadResult, wgPages *sync.WaitGroup) {
	for job := range jobs {
		/* download html -- job.Link */
		resp, errget := http.Get(job.Link)
		if errget != nil {
			log.Fatal("Error getting ", job.Link)
		}
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromResponse(resp)
		if err != nil {
			log.Fatal(err)
		}

		/* get image url */
		imageURL := sites[site].img(doc)

		/* download jpg */
		imageBytes := downloadImage(imageURL)

		/* send downloaded page to result channel */
		log.Printf("Chapter %d, Page %d done", job.Chapter, job.Page)
		downloadedPages <- DownloadResult{fmt.Sprintf("image-%03d-%03d.jpg", job.Chapter, job.Page), imageBytes}

		/* signal to downloadChapter that this page is done */
		wgPages.Done()
	}
}

func downloadChapter(site, manga string, chapters <-chan int, downloadedPages chan<- DownloadResult, numWorkers int, wgChapter *sync.WaitGroup) {
	for chapter := range chapters {

		baseurl := sites[site].url + manga

		/* get the first page & page list of the chapter */
		var pages []string
		links, pageImageBytes := getFirstPage(site, manga, baseurl, chapter)

		/* send the first page to the results channel */
		firstPageName := fmt.Sprintf("image-%03d-000.jpg", chapter)
		firstPageContent := pageImageBytes
		firstPage := DownloadResult{firstPageName, firstPageContent}
		downloadedPages <- firstPage

		/* gather the page html links from the links channel */
		for link := range links {
			pages = append(pages, sites[site].url+link)
		}

		/* channel for pages to download */
		jobs := make(chan DownloadJob)

		/* create pages waitgroup */
		var wgPages sync.WaitGroup
		wgPages.Add(len(pages) - 1) // first page is already done

		/* create workers */
		for i := 1; i <= numWorkers; i++ {
			go downloadPage(i, site, jobs, downloadedPages, &wgPages)
		}

		/* send jobs to worker channel, and close the channel */
		go func() {
			/* starting from the 2nd page onwards */
			for i := 1; i < len(pages); i++ {
				jobs <- DownloadJob{chapter, i, pages[i]}
			}
			close(jobs)
		}()

		/* wait until all pages are downloaded */
		wgPages.Wait()

		/* signal to downloadChapters that this chapter is done */
		log.Println("Chapter", chapter, "done")
		wgChapter.Done()
	}
}

func downloadChapters(site, manga string, fromChapter, toChapter, numChapterWorkers, numPageWorkers int) {
	/* get number of chapters from command line */
	numChapters := toChapter - fromChapter + 1 // +1 to include the starting chapter

	/* channel for chapters to be downloaded */
	chaptersJob := make(chan int)

	/* channel for downloaded pages */
	downloadedPages := make(chan DownloadResult)

	/* create waitgroup for chapter download work */
	var wgChapter sync.WaitGroup
	wgChapter.Add(numChapters)

	/* make workers */
	for i := 1; i <= numChapterWorkers; i++ {
		go downloadChapter(site, manga, chaptersJob, downloadedPages, numPageWorkers, &wgChapter)
	}

	/* send jobs to worker channel, and close the channel */
	go func() {
		for i := fromChapter; i <= toChapter; i++ {
			chaptersJob <- i
		}
		close(chaptersJob)
	}()

	/* send downloaded pages to cbz writer */
	var wgCBZ sync.WaitGroup
	wgCBZ.Add(1)
	cbzFile := fmt.Sprintf("%s-%03d-%03d.cbz", manga, fromChapter, toChapter)
	go createCBZchan(cbzFile, downloadedPages, &wgCBZ)

	/* wait for all chapter downloads */
	wgChapter.Wait()

	/* close the results channel, signaling createCBZchan process to clean up & terminate */
	close(downloadedPages)

	log.Println("All chapters done")

	/* wait until createCBZchan finished cleanly */
	wgCBZ.Wait()
}

func main() {
	startTime := time.Now()

	args := os.Args[1:]

	if len(args) < 3 {
		log.Fatal("Need <site> <name> <from> <to> parameters")
	}

	site := args[0]
	manga := args[1]
	from, _ := strconv.Atoi(args[2])
	to, _ := strconv.Atoi(args[3])

	parChapters := sites[site].parChapters
	parPages := sites[site].parPages

	log.Println("Parallel chapters:", parChapters, ", parallel pages:", parPages)
	log.Println(manga, from, to)

	downloadChapters(site, manga, from, to, parChapters, parPages)

	log.Println("Elapsed time:", time.Since(startTime))
}
