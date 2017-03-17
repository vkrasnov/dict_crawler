package bro

// #cgo CFLAGS: -I ./include/
// #cgo LDFLAGS: -lm
// #include <string.h>
// #include <stdlib.h>
// #include <brotli/encode.h>
import "C"
import (
	"compress/flate"
	"fmt"
	"io"
	"runtime"
	"unsafe"
)

const kFileBufferSize int = 65536

type BrotliEncoder struct {
	bro *C.struct_BrotliEncoderStateStruct
}

type Writer struct {
	w                       io.Writer
	bro                     *C.struct_BrotliEncoderStateStruct
	dict                    []byte
	writeBuffer, readBuffer *C.uint8_t
	nextIn, nextOut         *C.uint8_t
	availIn, availOut       C.size_t
	tmpBuff                 [kFileBufferSize]byte
}

func NewWriter(w io.Writer, level int) (*Writer, error) {
	var bw Writer

	if 0 > level || 11 < level {
		return nil, fmt.Errorf("brotli: invalid compression level %d: want value in range [0, 11]", level)
	}

	bw.bro = C.BrotliEncoderCreateInstance(nil, nil, nil)
	bw.writeBuffer = (*C.uint8_t)(C.malloc(C.size_t(kFileBufferSize)))
	bw.readBuffer = (*C.uint8_t)(C.malloc(C.size_t(kFileBufferSize)))

	if bw.bro == nil || bw.writeBuffer == nil || bw.readBuffer == nil {
		C.free((unsafe.Pointer(bw.writeBuffer)))
		C.free((unsafe.Pointer(bw.readBuffer)))
		C.BrotliEncoderDestroyInstance(bw.bro)
		return nil, flate.InternalError("cgo allocation failed")
	}

	bw.w = w
	bw.availIn = C.size_t(0)
	bw.nextIn = nil
	bw.availOut = C.size_t(kFileBufferSize)
	bw.nextOut = bw.writeBuffer
	C.BrotliEncoderSetParameter(bw.bro, C.BROTLI_PARAM_QUALITY, C.uint32_t(level))
	C.BrotliEncoderSetParameter(bw.bro, C.BROTLI_PARAM_LGWIN, C.BROTLI_DEFAULT_WINDOW)

	runtime.SetFinalizer(&bw, func(bw *Writer) {
		C.BrotliEncoderDestroyInstance(bw.bro)
		C.free(unsafe.Pointer(bw.writeBuffer))
		C.free(unsafe.Pointer(bw.readBuffer))
	})

	return &bw, nil
}

func (w *Writer) Write(data []byte) (n int, err error) {

	for {
		if int(w.availOut) != kFileBufferSize {
			outSize := kFileBufferSize - int(w.availOut)
			ptr := (*[kFileBufferSize]byte)(unsafe.Pointer(w.writeBuffer))

			m, e := w.w.Write(ptr[:outSize])
			if e != nil {
				return n, e
			}

			if m != outSize {
				w.availOut -= C.size_t(m)
				return n, fmt.Errorf("brotli:write pending")
			}

			w.availOut = C.size_t(kFileBufferSize)
			w.nextOut = w.writeBuffer
		}

		if int64(w.availIn) == 0 && len(data) != 0 {
			toCopy := len(data)
			if len(data) > kFileBufferSize {
				toCopy = kFileBufferSize
			}

			C.memcpy(unsafe.Pointer(w.readBuffer), unsafe.Pointer(&data[0]), C.size_t(toCopy))
			n += toCopy
			data = data[toCopy:]

			w.nextIn = w.readBuffer
			w.availIn = C.size_t(toCopy)
		}

		success := C.BrotliEncoderCompressStream(w.bro, C.BROTLI_OPERATION_PROCESS, &w.availIn, &w.nextIn, &w.availOut, &w.nextOut, nil)
		if !success {
			err = fmt.Errorf("brotli:failed to compress data")
			break
		}
	}

	return
}

func Encoder() *BrotliEncoder {
	bro := &BrotliEncoder{C.BrotliEncoderCreateInstance(nil, nil, nil)}
	return bro
}

func (b BrotliEncoder) SetDict(dict []byte, quality int) {
	C.BrotliEncoderSetParameter(b.bro, C.BROTLI_PARAM_QUALITY, C.uint32_t(quality))
	C.BrotliEncoderSetParameter(b.bro, C.BROTLI_PARAM_LGWIN, C.BROTLI_DEFAULT_WINDOW)
	C.BrotliEncoderSetCustomDictionary(b.bro, C.size_t(len(dict)), (*C.uint8_t)(unsafe.Pointer(&dict[0])))
}

func (b BrotliEncoder) Compress(quality int, in []byte) []byte {

	if in == nil {
		return nil
	}

	inSize := C.size_t(len(in))
	outSize := C.BrotliEncoderMaxCompressedSize(inSize)

	if len(in) == 0 {
		in = make([]byte, 1)
	}

	allocIn := C.malloc(inSize)
	allocOut := C.malloc(outSize)
	inCopy := (*C.uint8_t)(allocIn)
	outCopy := (*C.uint8_t)(allocOut)

	inPtr := unsafe.Pointer(&in[0])
	C.memcpy(unsafe.Pointer(inCopy), inPtr, inSize)

	availOut := outSize

	C.BrotliEncoderSetParameter(b.bro, C.BROTLI_PARAM_QUALITY, C.uint32_t(quality))
	C.BrotliEncoderSetParameter(b.bro, C.BROTLI_PARAM_LGWIN, C.BROTLI_DEFAULT_WINDOW)
	success := C.BrotliEncoderCompressStream(b.bro, C.BROTLI_OPERATION_FINISH,
		&inSize,
		&inCopy,
		&availOut,
		&outCopy,
		nil)

	out := make([]byte, outSize-availOut)
	outPtr := unsafe.Pointer(&out[0])
	C.memcpy(outPtr, allocOut, C.size_t(outSize-availOut))
	C.free(allocIn)
	C.free(allocOut)

	C.BrotliEncoderDestroyInstance(b.bro)
	b.bro = nil
	if success {
		return out[:outSize-availOut]
	} else {
		return nil
	}
}
