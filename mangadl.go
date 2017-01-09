package main

import (
	"archive/zip"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"

	"time"

	"github.com/PuerkitoBio/goquery"
)

// Site definition ...
type Site struct {
	url      string
	img      func(*goquery.Document) string
	pageList func(*goquery.Document) <-chan string
}

var mangareader = Site{
	url: "http://www.mangareader.net",
	img: func(doc *goquery.Document) string {
		/* <img id="img" src=[URL] name="img"> */
		imageURL, found := doc.Find("#img").Attr("src")
		if !found {
			log.Fatal("image not found")
		}
		return imageURL
	},
	pageList: func(doc *goquery.Document) <-chan string {
		/* <option value=[pagelist]> */
		links := make(chan string)
		go func() {
			doc.Find("option").Each(func(i int, s *goquery.Selection) {
				link, _ := s.Attr("value")
				links <- link
			})
			close(links)
		}()
		return links
	}}

var sites = map[string]Site{
	"mangareader": mangareader}

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

// DownloadResults ...
type DownloadResults []DownloadResult

func (d DownloadResults) Len() int {
	return len(d)
}

func (d DownloadResults) Less(i int, j int) bool {
	return d[i].Name < d[j].Name
}

func (d DownloadResults) Swap(i int, j int) {
	d[i], d[j] = d[j], d[i]
}

func downloadImage(url string) []byte {
	for retry := 1; retry <= 3; retry++ {
		/* open url */
		response, e := http.Get(url)
		if e != nil {
			log.Println("Error getting", url, "retrying...", retry)
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

func dumpImage(outfile string, imageBytes []byte) {
	writeErr := ioutil.WriteFile(outfile, imageBytes, 0644)
	if writeErr != nil {
		log.Fatal(writeErr)
	}
}

func createCBZ(cbzName string, files DownloadResults) {
	// Create a buffer to write our archive to.
	buf, createErr := os.Create(cbzName)
	if createErr != nil {
		log.Fatal(createErr)
	}

	// Create a new zip archive.
	w := zip.NewWriter(buf)

	// Add some files to the archive.
	log.Print("Creating cbz ")
	for _, file := range files {

		/* create zip writer with header of filename, STORE method, and current time */
		header := zip.FileHeader{
			Name:   file.Name,
			Method: zip.Store}
		header.SetModTime(time.Now())
		f, err := w.CreateHeader(&header)
		if err != nil {
			log.Fatal(err)
		}

		/* write downloaded content to zip archive */
		_, err = f.Write(file.Content)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Make sure to check the error on Close.
	err := w.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func getFirstPage(baseurl string, chapter int) (<-chan string, []byte) {
	/* get first page html */
	url := fmt.Sprintf("%s/%d", baseurl, chapter)
	doc, err := goquery.NewDocument(url)
	if err != nil {
		log.Fatal(err)
	}

	/* get first page image */
	pageImageURL := sites["mangareader"].img(doc)
	pageImageBytes := downloadImage(pageImageURL)

	/* get all pages links */
	links := sites["mangareader"].pageList(doc)

	return links, pageImageBytes
}

func downloadPage(n int, jobs <-chan DownloadJob, results chan<- DownloadResult) {
	for job := range jobs {
		/* download html -- job.Link */
		doc, err := goquery.NewDocument(job.Link)
		if err != nil {
			log.Fatal(err)
		}

		/* get image url */
		imageURL := sites["mangareader"].img(doc)

		/* download jpg */
		imageBytes := downloadImage(imageURL)

		/* send downloaded page to result channel */
		log.Printf("Worker %d: %+v done\n", n, job)
		results <- DownloadResult{fmt.Sprintf("image-%03d-%03d.jpg", job.Chapter, job.Page), imageBytes}
	}
}

func downloadChapter(site, manga string, chapters <-chan int, output chan<- DownloadResults, numWorkers int) {
	for chapter := range chapters {

		baseurl := site + "/" + manga

		/* get the first page & page list of the chapter */
		var pages []string
		links, pageImageBytes := getFirstPage(baseurl, chapter)
		for link := range links {
			pages = append(pages, site+link)
		}

		/* make channels */
		jobs := make(chan DownloadJob)
		results := make(chan DownloadResult, len(pages))

		/* create workers */
		for i := 1; i <= numWorkers; i++ {
			go downloadPage(i, jobs, results)
		}

		/* send jobs to worker channel, and close the channel */
		for i := 1; i < len(pages); i++ {
			jobs <- DownloadJob{chapter, i, pages[i]}
		}
		close(jobs)

		/* get the first page image */
		firstPageName := fmt.Sprintf("image-%03d-000.jpg", chapter)
		firstPageContent := pageImageBytes
		firstPage := DownloadResult{firstPageName, firstPageContent}
		downloadedImages := DownloadResults{firstPage}

		/* gather results from worker result channel */
		for i := 1; i < len(pages); i++ {
			downloadedImages = append(downloadedImages, <-results)
		}
		output <- downloadedImages

		log.Println("Chapter", chapter, "done")
	}
}

func downloadChapters(site, manga string, fromChapter, toChapter, numChapterWorkers, numPageWorkers int) {
	/* get number of chapters from command line */
	numChapters := toChapter - fromChapter + 1 // +1 to include the starting chapter

	/* make channels */
	chaptersJob := make(chan int)
	downloadedChapter := make(chan DownloadResults, numChapters)

	/* make workers */
	for i := 1; i <= numChapterWorkers; i++ {
		go downloadChapter(site, manga, chaptersJob, downloadedChapter, numPageWorkers)
	}

	/* send jobs to worker channel, and close the channel */
	for i := fromChapter; i <= toChapter; i++ {
		chaptersJob <- i
	}
	close(chaptersJob)

	/* gather results from worker result channel */
	var results DownloadResults
	for i := 1; i <= numChapters; i++ {
		results = append(results, <-downloadedChapter...)
	}

	/* put results into cbz file */
	cbzFile := fmt.Sprintf("%s-%03d-%03d.cbz", manga, fromChapter, toChapter)
	createCBZ(cbzFile, results)
}

func main() {
	startTime := time.Now()

	parChapters := 6
	parPages := 6

	args := os.Args[1:]

	if len(args) < 3 {
		log.Fatal("Need <name> <from> <to> parameters")
	}
	manga := args[0]
	from, _ := strconv.Atoi(args[1])
	to, _ := strconv.Atoi(args[2])

	log.Println(manga, from, to)

	mangareader := "http://www.mangareader.net"
	downloadChapters(mangareader, manga, from, to, parChapters, parPages)

	log.Println("Elapsed time:", time.Since(startTime))
}
