package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync/atomic"

	"github.com/secsy/goftp"
)

func main() {
	ExampleClient_ReadDir_parallelWalk()
}

// Just for fun, walk an ftp server in parallel. I make no claim that this is
// correct or a good idea.
func ExampleClient_ReadDir_parallelWalk() {
	config := goftp.Config{
		User:     "free",
		Password: "free",
	}
	client, err := goftp.DialConfig(config, "ftp.zakupki.gov.ru")
	if err != nil {
		panic(err)
	}

	Walk(client, "/fcs_regions/Moskva", func(fullPath string, info os.FileInfo, err error) error {
		if err != nil {
			// no permissions is okay, keep walking
			if err.(goftp.Error).Code() == 550 {
				return nil
			}
			return err
		}

		fmt.Println(fullPath)

		return nil
	})
}

// Walk a FTP file tree in parallel with prunability and error handling.
// See http://golang.org/pkg/path/filepath/#Walk for interface details.
func Walk(client *goftp.Client, root string, walkFn filepath.WalkFunc) (ret error) {
	dirsToCheck := make(chan string, 100)

	var workCount int32 = 1
	dirsToCheck <- root

	for dir := range dirsToCheck {
		go func(dir string) {
			files, err := client.ReadDir(dir)

			if err != nil {
				if err = walkFn(dir, nil, err); err != nil && err != filepath.SkipDir {
					ret = err
					close(dirsToCheck)
					return
				}
			}

			for _, file := range files {
				p := path.Join(dir, file.Name())

				if err = walkFn(p, file, nil); err != nil {
					if file.IsDir() && err == filepath.SkipDir {
						continue
					}

					ret = err
					close(dirsToCheck)
					return
				}

				if file.IsDir() {
					atomic.AddInt32(&workCount, 1)
					dirsToCheck <- path.Join(dir, file.Name())
				} else {
					buf := new(bytes.Buffer)
					client.Retrieve(p, buf)
					unzip(buf)
				}
			}

			atomic.AddInt32(&workCount, -1)
			if workCount == 0 {
				close(dirsToCheck)
			}
		}(dir)
	}

	return ret
}

func unzip(buf *bytes.Buffer) {
	rb := bytes.NewReader(buf.Bytes())
	r, _ := zip.NewReader(rb, int64(rb.Len()))
	for _, f := range r.File {
		println(f.Name)

		rc, err := f.Open()
		if err != nil {
			panic(err)
		}
		strbuf := new(bytes.Buffer)
		strbuf.ReadFrom(rc)
		println(strbuf.String())
		if err != nil {
			panic(err)
		}
		parseXML(strbuf.String())
		rc.Close()
		//fmt.Println()
		panic(1)
	}

}

type ikz struct {
	IKZs []string `xml:"fcsNotificationEF>purchaseNumber"` //`xml:"fcsPurchaseDocument>purchaseNumber"`
}

func parseXML(data string) {
	v := ikz{IKZs: []string{}}
	xml.Unmarshal([]byte(data), &v)
	fmt.Printf("Names of people: %q", v)

}
