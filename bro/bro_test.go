package bro

import (
	"bytes"
	"log"
	"testing"
)

var brotliTests = []struct {
	dict, in string
}{
	{dict: "", in: ""},
	{dict: "", in: "XXXXXXXXXXYYYYYYYYYY"},
	{dict: "this is a brotli test", in: "brotli test here"},
}

func TestBrotli(t *testing.T) {

	for _, v := range brotliTests {
		enc := Encoder()
		dec := Decoder()

		if v.dict != "" {
			enc.SetDict([]byte(v.dict))
			dec.SetDict([]byte(v.dict))
		}

		comp := enc.Compress(12, []byte(v.in))
		log.Println(comp)
		decomp := dec.Decompress(comp)

		if !bytes.Equal(decomp, []byte(v.in)) {
			t.Errorf("Error in test %v", v)
		}
	}
}
