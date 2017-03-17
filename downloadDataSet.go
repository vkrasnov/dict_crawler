package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/fedesog/webdriver"
)

var desired, required webdriver.Capabilities
var chromeDriver *webdriver.ChromeDriver

func getHttp2UrlList(start, end int) map[string]bool {
	http2List, err := http.Get(http2Url)
	if err != nil {
		log.Println(err)
	}
	defer http2List.Body.Close()

	list := make(map[string]bool)

	r := bufio.NewReader(http2List.Body)

	for i := 0; i < end; i++ {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}
		if i < start {
			continue
		}
		list[strings.TrimSuffix(line, "\n")] = true
	}
	return list
}

type readerAt struct {
	body []byte
}

func (r readerAt) ReadAt(p []byte, off int64) (n int, err error) {
	n = copy(p, r.body[off:])

	if n < len(p) {
		err = fmt.Errorf("end of buffer")
	}
	return
}

func getAlexaUrlList(start, end int) map[string]bool {
	top, err := http.Get("http://s3.amazonaws.com/alexa-static/top-1m.csv.zip")
	if err != nil {
		log.Fatalln("Failed to download URL list", err)
	}
	defer top.Body.Close()

	list := make(map[string]bool)

	body, err := ioutil.ReadAll(top.Body)
	if err != nil {
		log.Fatalln("Failed to download URL list", err)
	}

	zipped := readerAt{body}
	r, err := zip.NewReader(zipped, int64(len(body)))
	if err != nil {
		log.Fatalln("Failed to unzip URL list", err)
	}

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			log.Fatal(err)
		}
		br := bufio.NewReader(rc)

		for i := 0; i < end; i++ {
			line, err := br.ReadString('\n')
			if err != nil {
				log.Println(err)
				break
			}

			if i < start {
				continue
			}

			host := strings.Split(line, ",")[1]
			list[strings.TrimSuffix(host, "\n")] = true
		}
		rc.Close()
	}

	return list
}

func getLinks(session *webdriver.Session, site string) []string {
	ret := make([]string, 0)

	thisIps, err := net.LookupIP(strings.Split(strings.TrimPrefix(strings.TrimPrefix(site, "http://"), "https://"), "/")[0])
	if err != nil {
		log.Println(err)
		return ret
	}
	// Get all links on the rendered page
	elements, err := session.FindElements("partial link text", "")
	if err != nil {
		log.Println(err)
		return ret
	}

	for i, a := range elements {
        if i > 50 {
            break
        }
		// In this simplistic analysis we only chek hrefs
		link, err := a.GetAttribute("href")
		if err != nil {
			continue
		}
		// Parse the link
		parsedLink, err := url.Parse(link)
		if err != nil {
			continue
		}
		// Check that the link is relative or http/https
		if parsedLink.Scheme != "" && parsedLink.Scheme != "http" && parsedLink.Scheme != "https" {
			continue
		}
		// If host is not empty, check that it resolves to the same ip
		if parsedLink.Host != "" {
			matchIP := false
			ips, err := net.LookupIP(parsedLink.Host)
			if err != nil {
				continue
			}
			for _, ip := range ips {
				if bytes.Equal(ip, thisIps[0]) {
					matchIP = true
					break
				}
			}
			if !matchIP {
				continue
			}
		}
		// If the host is the same, do not allow index.html to be loaded twice
		if parsedLink.Host == strings.TrimPrefix(strings.TrimPrefix(site, "https://"), "http://") || parsedLink.Host == "" {
			if parsedLink.Path == "" || parsedLink.Path == "/" || parsedLink.Path == "/index.html" {
				continue
			}
		}

		if parsedLink.Host == "" {
			parsedLink.Host = site
		}
		ret = append(ret, parsedLink.String())
	}

	dest := make([]string, len(ret))
	perm := rand.Perm(len(ret))
	for i, v := range perm {
		dest[v] = ret[i]
	}
	return dest
}

func downloadDataSet(address string, clicks int) {
	log.Print(address)

	resp, err := http.Head("http://" + address)
	if err != nil {
		log.Println(err)
		resp, err = http.Head("http://www." + address)
		if err != nil {
			log.Println(err)
			return
		}
	}
	realAddress := resp.Request.URL.String()
	resp.Body.Close()
	log.Println("Final address:", realAddress)

	// We need the ips for later
	// Start new session
	session, err := chromeDriver.NewSession(desired, required)
	if err != nil {
		log.Println(err)
		return
	}

	defer session.Delete()
	// Don't wait more than that on any webpage
	err = session.SetTimeouts("page load", 20 * 1000)
	// Try to navigate to the page
	err = session.Url(realAddress)
	if err != nil {
		log.Println("Navigation error:", err)
	    return
    }

	links := getLinks(session, realAddress)
	for i, l := range links {
		if i == clicks {
			break
		}
		log.Println("Click on: ", l)
		err = session.Url(l)
		if err != nil {
			log.Println("Navigation error:", err)
		}
	}

	// Performance log contains the network information
	logText, err := session.Log("performance")
	if err != nil {
		log.Println("Error getting performance log:", err)
		return
	}

	path := datapath + address
	err = os.MkdirAll(path, 0777)
	if err != nil {
		log.Println("Could not create dir", err)
	}

	var IPaddr string
	count := 0
	manifest := ""
	client := &http.Client{}

	for _, l := range logText {
		var val map[string]interface{}

		if err := json.Unmarshal([]byte(l.Message), &val); err != nil {
			log.Println(err)
			continue
		}

		if msg, ok := val["message"]; !ok {
			continue
		} else if method, ok := msg.(map[string]interface{})["method"]; !ok || method != "Network.responseReceived" {
			continue
		} else if params, ok := msg.(map[string]interface{})["params"]; !ok {
			continue
		} else if response, ok := params.(map[string]interface{})["response"]; !ok {
			continue
		} else {
			thisAddress, ok := response.(map[string]interface{})["remoteIPAddress"].(string)
			if !ok {
				continue
			}

			if IPaddr == "" {
				IPaddr = thisAddress
			} else if IPaddr != thisAddress {
				continue
			}

			request, ok := response.(map[string]interface{})["requestHeaders"].(map[string]interface{})
			if !ok {
				continue

			}
			if method, ok := request[":method"]; ok {
				if method != "GET" {
					continue
				}
			} else if requestText, ok := response.(map[string]interface{})["requestHeadersText"].(string); ok {
				if !strings.HasPrefix(requestText, "GET") {
					continue
				}
			} else {
				continue
			}

			thisUrl, ok := response.(map[string]interface{})["url"].(string)
			if !ok {
				continue
			}

			responseHeaders, ok := response.(map[string]interface{})["headers"].(map[string]interface{})
			if !ok {
				continue
			}

			var contentType string
			if contentType, ok = responseHeaders["content-type"].(string); !ok {
				if contentType, ok = responseHeaders["Content-Type"].(string); !ok {
					if contentType, ok = responseHeaders["Content-type"].(string); !ok {
						continue
					}
				}
			}
			log.Println(thisUrl, contentType)

			if _, ok := acceptedContent[strings.Split(contentType, ";")[0]]; !ok {
				if !strings.HasPrefix(contentType, "image") &&
					!strings.HasPrefix(contentType, "multipart") &&
					!strings.HasSuffix(contentType, "woff") {
					log.Println("Invalid Content Type for compression ", contentType)
				}
				continue
			}
			/* Download the asset for analysis */
			req, err := http.NewRequest("GET", thisUrl, nil)

			if err != nil {
				log.Println(err)
				continue
			}

			if request, ok := response.(map[string]interface{})["requestHeaders"]; ok {
				for k, v := range request.(map[string]interface{}) {
					if strings.ToLower(k) == "user-agent" || strings.ToLower(k) == "cookie" {
						req.Header.Add(k, v.(string))
					}
				}
			}

			res, err := client.Do(req)
			if err != nil {
				log.Println(err)
				continue
			}

			defer res.Body.Close()
			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				log.Println(err)
				continue
			}

			if len(body) == 0 {
				log.Println(len(body))
				continue
			}

			err = ioutil.WriteFile(path+"/"+strconv.Itoa(count), body, 0777)
			if err != nil {
				log.Println(err)
				continue
			}

			manifest += thisUrl + "{{{{" + contentType + "}}}}" + strconv.Itoa(len(body)) + "\n"
			count++
		}
		ioutil.WriteFile(path+"/manifest", []byte(manifest), 0777)
	}
}

func download(webSites map[string]bool) {
	chromeDriver = webdriver.NewChromeDriver(*chromedriverpath + "/chromedriver")
	//chromeDriver.LogFile = "error.log"
	err := chromeDriver.Start()
	if err != nil {
		log.Println(err)
	}
	defer chromeDriver.Stop()

	logCapabilities := webdriver.Capabilities{
		"browser":     "OFF",
		"performance": "INFO",
	}

	perfLogCapabilities := webdriver.Capabilities{
		"enableNetwork":  true,
		"enablePage":     false,
		"enableTimeline": false,
	}

	args := webdriver.Capabilities{
		//"args": []string{"incognito", "disable-http2"},
		"args": []string{"ignore-certificate-errors", "incognito", "window-position=22220,22220", "window-size=1,1"},
	}

	desired = webdriver.Capabilities{
		"Platform":         "Linux",
		"loggingPrefs":     logCapabilities,
		"perfLoggingPrefs": perfLogCapabilities,
		"chromeOptions":    args,
	}

	required = webdriver.Capabilities{}

	for site := range webSites {
		downloadDataSet(site, *clicks)
	}
}
