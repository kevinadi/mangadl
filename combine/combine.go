package combine

import (
	"archive/zip"
	"io/ioutil"
	"log"
	"os"
	"sync"
)

type zipFile struct {
	Header  zip.FileHeader
	Content []byte
}

func readZip(fileName string, contents chan<- zipFile, wg *sync.WaitGroup) {
	/* open the zip file */
	r, err := zip.OpenReader(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	/* loop through all contents */
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			log.Fatal(err)
		}

		bytes, readErr := ioutil.ReadAll(rc)
		if readErr != nil {
			log.Fatal(readErr)
		}
		contents <- zipFile{
			Header:  f.FileHeader,
			Content: bytes}

		rc.Close()
	}
	/* signal combine that reads are done */
	wg.Done()
}

func combineCBZchan(cbzName string, contents <-chan zipFile, wgCBZ *sync.WaitGroup) {
	/* create the zip file */
	buf, createErr := os.Create(cbzName)
	if createErr != nil {
		log.Fatal(createErr)
	}
	zipWriter := zip.NewWriter(buf)
	log.Println("Creating cbz:", cbzName)

	/* write to zipfile */
	for file := range contents {
		/* get zip header */
		header := file.Header
		f, err := zipWriter.CreateHeader(&header)
		if err != nil {
			log.Fatal("CreateHeader: ", err)
		}

		/* write content to zip archive */
		_, err = f.Write(file.Content)
		if err != nil {
			log.Fatal(err)
		}

		log.Println("Added:", header.Name)
	}

	/* close the archive */
	err := zipWriter.Close()
	if err != nil {
		log.Fatal(err)
	}
	log.Println(cbzName, "closed")

	/* signal combine that all writes are done and the archive is closed */
	wgCBZ.Done()
}

func Combine() {
	args := os.Args[2:]
	log.Println("Args: ", args)

	contents := make(chan zipFile)
	var wg sync.WaitGroup
	wg.Add(len(args) - 1)

	/* read from multiple files concurrently */
	for _, fileName := range args[1:] {
		go readZip(fileName, contents, &wg)
	}

	/* combine output channel */
	var wgCBZ sync.WaitGroup
	wgCBZ.Add(1)
	go combineCBZchan(args[0], contents, &wgCBZ)

	/* wait until everything is finished */
	wg.Wait()
	close(contents)
	wgCBZ.Wait()
}
