package main

import "flag"

var dictSize int = 32768

var DeflateCompressionLevel int = 6
var BrotliCompressionLevel int = 4

var alexaUrl = "http://s3.amazonaws.com/alexa-static/top-1m.csv.zip"
var http2Url = "http://isthewebhttp2yet.com/data/lists/H2-true-2016-10-09.txt"

var datapath = "./dataset/"
var dictpath = "./dicts/"

var acceptedContentShort = map[string]bool{
	"text/html":                true,
	"text/css":                 true,
	"text/javascript":          true,
	"text/plain":               true,
	"application/javascript":   true,
	"application/json":         true,
	"application/x-javascript": true,
}

var acceptedContent = map[string]bool{
	"text/html":                     true,
	"text/richtext":                 true,
	"text/plain":                    true,
	"text/css":                      true,
	"text/x-script":                 true,
	"text/x-component":              true,
	"text/x-java-source":            true,
	"text/x-markdown":               true,
	"application/javascript":        true,
	"application/x-javascript":      true,
	"text/javascript":               true,
	"text/js":                       true,
	"image/x-icon":                  true,
	"application/x-perl":            true,
	"application/x-httpd-cgi":       true,
	"text/xml":                      true,
	"application/xml":               true,
	"application/xml+rss":           true,
	"application/json":              true,
	"multipart/bag":                 true,
	"multipart/mixed":               true,
	"application/xhtml+xml":         true,
	"font/ttf":                      true,
	"font/otf":                      true,
	"font/x-woff":                   true,
	"image/svg+xml":                 true,
	"application/vnd.ms-fontobject": true,
	"application/ttf":               true,
	"application/x-ttf":             true,
	"application/otf":               true,
	"application/x-otf":             true,
	"application/truetype":          true,
	"application/opentype":          true,
	"application/x-opentype":        true,
	"application/font-woff":         true,
	"application/eot":               true,
	"application/font":              true,
	"application/font-sfnt":         true,
}

var doDownload = flag.Bool("d", false, "Download the dataset")
var useAlexa = flag.Bool("a", true, "Use Alexa top for dataset (otherwise use isthewebhttp2yet dataset)")
var doGenDict = flag.Bool("dict", false, "Generate new shared dictionaries")
var doCompressionTest = flag.Bool("c", false, "Perform compression test")
var dataSetSize = flag.Int("n", 200, "How many websites to put into the dataset")
var bl = flag.Int("bl", 4, "Brotli level")
var dl = flag.Int("dl", 6, "Deflate level")
var custom = flag.String("w", "", "download some other website instead")
var ds = flag.Int("ds", 32768, "size of the dictionary to use")
var chromedriverpath = flag.String("cd", ".", "path to chromedriver")
var dsp = flag.String("dataset", "./dataset/", "path to dataset")
var dp = flag.String("dicts", "./dicts/", "path to dictionaries")
var skip = flag.Int("skip", 0, "skip directories that have at most this many files")
var clicks = flag.Int("clicks", 1, "How many \"clicks\" to simulate during download")
var xlsxpath = flag.String("x", "./output.xlsx", "Where to save the xlsx file")

func main() {
	flag.Parse()

	dictSize = *ds
	datapath = *dsp
	dictpath = *dp

	if datapath == "" {
		return
	}

	if datapath[len(datapath)-1] != '/' {
		datapath = datapath + "/"
	}

	if *doDownload {
		var webSites map[string]bool
		if *custom != "" {
			webSites = map[string]bool{*custom: true}
		} else {
			if *useAlexa {
				webSites = getAlexaUrlList(0, *dataSetSize)
			} else {
				webSites = getHttp2UrlList(0, *dataSetSize)
			}
		}
		download(webSites)
	}

	BrotliCompressionLevel = *bl
	DeflateCompressionLevel = *dl

	if *doGenDict {
		genSharedDictionaries()
	}

	if *doCompressionTest {
		testStrategy()
	}
}
