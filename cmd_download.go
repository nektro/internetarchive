package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nektro/internetarchive/pkg/cmd"
	. "github.com/nektro/internetarchive/pkg/util"

	"github.com/PuerkitoBio/goquery"
	"github.com/nektro/go-util/mbpp"
	"github.com/nektro/go-util/util"
	"github.com/spf13/cobra"
)

func init() {
	cmdDownload.Flags().StringP("save-dir", "o", "./data", "")
	cmdDownload.Flags().Bool("only-meta", false, "")
	cmdDownload.Flags().Bool("dense", false, "")
	cmd.Root.AddCommand(cmdDownload)
}

var cmdDownload = &cobra.Command{
	Use:   "download",
	Short: "download an item or collection",
	Run: func(c *cobra.Command, args []string) {
		Assert(len(args) > 0, "missing item identifier")
		p, _ := c.Flags().GetString("save-dir")
		om, _ := c.Flags().GetBool("only-meta")
		dn, _ := c.Flags().GetBool("dense")
		d, _ := filepath.Abs(p)
		mbpp.Init(10)
		dlItem(d, args[0], nil, om, dn)
		mbpp.Wait()
		time.Sleep(time.Second)
		log.Println(mbpp.GetCompletionMessage())
	},
}

func dlItem(dir, name string, b *mbpp.BarProxy, onlyMeta, dense bool) {
	mbpp.CreateJob("item: "+name, func(bar *mbpp.BarProxy) {
		bar.AddToTotal(2)
		doc, bys, ok := GetDoc("https://archive.org/download/"+name+"/"+name+"_meta.xml", nil)
		bar.Increment(1)
		if !ok {
			bar.Increment(1)
			return
		}
		mt := doc.Find("mediatype").Text()
		if mt == "collection" {
			bar.Increment(1)
			go dlCollection(dir, name, onlyMeta, dense)
			return
		}
		ad := doc.Find("addeddate").Text()
		ad = ad[:strings.IndexRune(ad, ' ')]
		ad = strings.ReplaceAll(ad, "-", "/")
		dir2 := dir
		if dense {
			dir2 += "/" + ad
		}
		dir2 += "/" + name
		if util.DoesDirectoryExist(dir2) {
			bar.Increment(1)
			return
		}
		if onlyMeta {
			os.MkdirAll(dir2, os.ModePerm)
			f, _ := os.Create(dir2 + "/" + name + "_meta.xml")
			io.Copy(f, bytes.NewReader(bys))
			bar.Increment(1)
			return
		}
		doc2, _, _ := GetDoc("https://archive.org/download/"+name+"/"+name+"_files.xml", nil)
		bar.Increment(1)
		arr := doc2.Find("file")
		arr.Each(func(_ int, el *goquery.Selection) {
			n, _ := el.Attr("name")
			s, _ := el.Attr("source")
			if s != "original" {
				return
			}
			bar.AddToTotal(1)
			go saveTo(dir2, name, n, bar)
		})
	})
}

func dlCollection(dir, name string, onlyMeta, dense bool) {
	mbpp.CreateJob("collection: "+name, func(bar *mbpp.BarProxy) {
		dat := map[string]string{"x-requested-with": "XMLHttpRequest"}
		for i := 1; true; i++ {
			doc, _, _ := GetDoc("https://archive.org/details/"+name+"?&page="+strconv.Itoa(i), dat)
			arr := doc.Find(".item-ia[data-id]")
			if arr.Length() == 1 {
				break
			}
			arr.Each(func(_ int, el *goquery.Selection) {
				n, _ := el.Attr("data-id")
				if n == "__mobile_header__" {
					return
				}
				if onlyMeta {
					go dlItem(dir, n, bar, onlyMeta, dense)
					return
				}
				dlItem(dir, n, bar, onlyMeta, dense)
			})
		}
	})
}

func saveTo(dir, item, file string, b *mbpp.BarProxy) {
	pathS := dir + "/" + file
	os.MkdirAll(filepath.Dir(pathS), os.ModePerm)
	urlS := "https://archive.org/download/" + item + "/" + file
	mbpp.CreateDownloadJob(urlS, pathS, b)
}
