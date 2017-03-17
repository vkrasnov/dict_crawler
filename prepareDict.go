package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/vkrasnov/dictator"
)

type asset struct {
	idx         int
	path        string
	contentType string
	content     []byte
}

var manifestRE *regexp.Regexp

func init() {
	manifestRE = regexp.MustCompile("(?i)" + "([^{]*[{]{0,3}){{{{([^}]*)}}}}([0-9]*)[\n]?")
}

func parseManifest(path string) []*asset {
	ret := make([]*asset, 0)

	manifest, _ := ioutil.ReadFile(path)
	sm := manifestRE.FindSubmatch(manifest)
	idx := 0
	for len(sm) == 4 {
		ct := strings.Split(string(sm[2]), ";")
		ret = append(ret, &asset{idx: idx, path: string(sm[1]), contentType: ct[0], content: nil})
		manifest = manifest[len(sm[0]):]
		sm = manifestRE.FindSubmatch(manifest)
		idx++
	}

	return ret
}

func genSharedDictionaries() {
	dirs, _ := ioutil.ReadDir(datapath)

	fileByType := make(map[string]*[]string)

	for _, d := range dirs {
		man := parseManifest(datapath + d.Name() + "/manifest")
		for _, m := range man {
			files := fileByType[m.contentType]
			if files == nil {
				f := make([]string, 0)
				files = &f
				fileByType[m.contentType] = files
			}
			*files = append(*files, datapath+d.Name()+"/"+strconv.Itoa(m.idx))
		}
	}

	if len(fileByType) > 0 {
		os.MkdirAll(dictpath, 0777)
	}

	for name, paths := range fileByType {
		log.Println(name)
		progress := make(chan float64, len(*paths))
		go func() {
			for percent := range progress {
				fmt.Printf("\r%.2f%% ", percent)
			}
		}()
		table := dictator.GenerateTable(dictSize/2, *paths, DeflateCompressionLevel, progress, 4)
		fmt.Println("\r100%  ")
		fmt.Println("Total incompressible strings found: ", len(table))

		dictionary := dictator.GenerateDictionary(table, dictSize, int(math.Ceil(float64(len(*paths))*0.01)))
		name = strings.Replace(name, "/", "__", -1)
		err := ioutil.WriteFile(dictpath+name+".dict", []byte(dictionary), 0644)
		if err != nil {
			log.Println(err)
		}
	}
}
