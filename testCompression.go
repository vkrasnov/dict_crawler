package main

import (
	"./bro"
	"bytes"
	"compress/flate"
	"fmt"
	"github.com/tealeg/xlsx"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
)

type compressor interface {
	String() string
	CompressWithDict([]byte, []byte, int) []byte
}

type gzipper struct {
}

type brotler struct {
}

var compressors []compressor = []compressor{&gzipper{}, &brotler{}}

func (c *brotler) String() string {
	return "Brotli"
}

func (c *gzipper) String() string {
	return "Deflate"
}

func (c *brotler) CompressWithDict(in, dict []byte, quality int) []byte {
	brot := bro.Encoder()

	if dict != nil {
		brot.SetDict(dict, quality)
	}

	return brot.Compress(quality, in)
}

func (c *gzipper) CompressWithDict(in, dict []byte, quality int) []byte {
	var def *flate.Writer
	var b bytes.Buffer

	if dict != nil {
		def, _ = flate.NewWriterDict(&b, quality, dict)
	} else {
		def, _ = flate.NewWriter(&b, quality)
	}

	def.Write(in)
	def.Close()
	return b.Bytes()
}

var strategies []func([]*asset, compressor, int) int

func init() {
	strategies = make([]func([]*asset, compressor, int) int, 0)
	strategies = append(strategies, strategy0)
	strategies = append(strategies, strategy1)
	strategies = append(strategies, strategy2)
	strategies = append(strategies, strategy3)
	strategies = append(strategies, strategy4)
	strategies = append(strategies, strategy5)
	strategies = append(strategies, strategy6)
	strategies = append(strategies, strategy7)
}

/* This one is the reference: simply compress */
func strategy0(list []*asset, c compressor, quality int) int {
	ret := 0

	for _, u := range list {
		ret += len(c.CompressWithDict(u.content, nil, quality))
	}

	return ret
}

/* Use the first stream, always */
func strategy1(list []*asset, c compressor, quality int) int {
	ret := 0
	var dict []byte

	for _, u := range list {
		ret += len(c.CompressWithDict(u.content, dict, quality))

		if dict == nil {
			if len(u.content) > dictSize {
				dict = u.content[:dictSize]
			} else {
				dict = u.content
			}
		}
	}

	return ret
}

/* Use the previous stream, always */
func strategy2(list []*asset, c compressor, quality int) int {
	ret := 0
	var dict []byte

	for _, u := range list {
		ret += len(c.CompressWithDict(u.content, dict, quality))

		if len(u.content) > dictSize {
			dict = u.content[:dictSize]
		} else {
			dict = u.content
		}
	}

	return ret
}

func toDictSize(in []byte) []byte {
	if len(in) > dictSize {
		return in[:dictSize]
	} else {
		return in
	}
}

func toDictSizeFromEnd(in []byte) []byte {
	if len(in) > dictSize {
		return in[len(in)-dictSize:]
	} else {
		return in
	}
}

/* Use the concatenation of all previous streams as dictionary */
func strategy3(list []*asset, c compressor, quality int) int {
	ret := 0
	var dict []byte

	for _, u := range list {
		ret += len(c.CompressWithDict(u.content, dict, quality))

		if dict == nil {
			dict = toDictSize(u.content)
		} else {
			dict = toDictSizeFromEnd(append(dict, toDictSize(u.content)...))
		}
	}

	return ret
}

/* Use last stream with the same content type as dictionary, otherwise use the first stream */
func strategy4(list []*asset, c compressor, quality int) int {
	ret := 0
	dicts := make(map[string][]byte)
	var firstDict []byte

	for _, u := range list {
		var dict []byte

		if firstDict == nil {
			firstDict = toDictSize(u.content)
		} else if getDict, ok := dicts[u.contentType]; ok {
			dict = getDict
		} else {
			dict = firstDict
		}

		dicts[u.contentType] = toDictSize(u.content)

		ret += len(c.CompressWithDict(u.content, dict, quality))
	}

	return ret
}

func openDicts() map[string][]byte {
	dicts, _ := ioutil.ReadDir(dictpath)

	dictByType := make(map[string][]byte)

	for _, d := range dicts {
		dict, _ := ioutil.ReadFile(dictpath + d.Name())
		ct := strings.TrimSuffix(strings.Replace(d.Name(), "__", "/", -1), ".dict")
		dictByType[ct] = dict
	}

	return dictByType
}

/* Use content type based static dictionary */
func strategy5(list []*asset, c compressor, quality int) int {
	ret := 0
	dicts := openDicts()

	for _, u := range list {
		var dict []byte

		if d, ok := dicts[u.contentType]; ok {
			dict = d
		}

		ret += len(c.CompressWithDict(u.content, dict, quality))
	}

	return ret
}

/* Use content type based static + dynamic dictionary */
func strategy6(list []*asset, c compressor, quality int) int {
	ret := 0
	dicts := openDicts()

	for _, u := range list {
		var dict []byte

		if d, ok := dicts[u.contentType]; ok {
			dict = d
		}

		ret += len(c.CompressWithDict(u.content, dict, quality))

		dicts[u.contentType] = toDictSize(u.content)
	}
	return ret
}

/* Use content type based static+dynamic "rolling" dictionary */
func strategy7(list []*asset, c compressor, quality int) int {
	dicts := openDicts()
	var dict []byte
	ret := 0

	for _, u := range list {

		if d, ok := dicts[u.contentType]; ok {
			dict = d
		}

		ret += len(c.CompressWithDict(u.content, dict, quality))

		dict = toDictSizeFromEnd(append(dict, toDictSize(u.content)...))
		dicts[u.contentType] = dict
	}
	return ret
}

func testStrategy() {
	/* For each compression algorithm and each stratgy we will have own sheet */
	file := xlsx.NewFile()
	sheets := make([]*xlsx.Sheet, len(strategies)*len(compressors))

	for i, c := range compressors {
		for j, _ := range strategies {
			sheets[i*len(strategies)+j], _ = file.AddSheet(fmt.Sprintf("%s, S%d", c.String(), j))
			row := sheets[i*len(strategies)+j].AddRow()
			row.AddCell().Value = "Website"
			for q := 4; q <= 8; q++ {
				row.AddCell().Value = "Quality" + strconv.Itoa(q)
			}
		}
	}

	dirs, _ := ioutil.ReadDir(datapath)
	for _, d := range dirs {
		man := parseManifest(datapath + d.Name() + "/manifest")

		if len(man) <= *skip {
			continue
		}

		for _, m := range man {
			content, err := ioutil.ReadFile(datapath + d.Name() + "/" + strconv.Itoa(m.idx))
			if err != nil {
				log.Print(err)
			}
			m.content = content
		}

		for i, c := range compressors {
			for j, s := range strategies {

				workingSheet := sheets[i*len(strategies)+j]
				row := workingSheet.AddRow()
				row.AddCell().Value = d.Name()

				for quality := 4; quality <= 8; quality++ {
					log.Println(d.Name(), quality, c)
					res := s(man, c, quality)
					row.AddCell().SetInt(res)
				}
			}
		}
		file.Save(*xlsxpath)
	}
}
