package bro

// #include <string.h>
// #include <stdlib.h>
// #include <brotli/decode.h>
import "C"
import "unsafe"

type BrotliDecoder struct {
	bro *C.struct_BrotliDecoderStateStruct
}

func Decoder() BrotliDecoder {
	return BrotliDecoder{C.BrotliDecoderCreateInstance(nil, nil, nil)}
}

func (b BrotliDecoder) SetDict(dict []byte) {
	C.BrotliDecoderSetCustomDictionary(b.bro, C.size_t(len(dict)), (*C.uint8_t)(unsafe.Pointer(&dict[0])))
}

func (b BrotliDecoder) Decompress(in []byte) []byte {
	if in == nil || len(in) == 0 {
		return nil
	}

	inSize := C.size_t(len(in))
	inPtr := unsafe.Pointer(&in[0])
	allocBuf := C.malloc(C.size_t(inSize))
	inCopy := (*C.uint8_t)(allocBuf)
	C.memcpy(unsafe.Pointer(inCopy), inPtr, C.size_t(inSize))

	outSize := C.size_t(len(in) * 100)
	availOut := outSize

	outCopy := (*C.uint8_t)(C.malloc(C.size_t(outSize)))
	outSave := outCopy

	success := C.BrotliDecoderDecompressStream(b.bro,
		(*C.size_t)(unsafe.Pointer(&inSize)),
		&inCopy,
		(*C.size_t)(unsafe.Pointer(&availOut)),
		&outCopy,
		nil)

	out := make([]byte, outSize-availOut)

	if len(out) != 0 {
		outPtr := unsafe.Pointer(&out[0])
		C.memcpy(outPtr, unsafe.Pointer(outSave), C.size_t(outSize-availOut))
		C.free(allocBuf)
		C.BrotliDecoderDestroyInstance(b.bro)
		b.bro = nil
	}

	if int(success) == C.BROTLI_DECODER_RESULT_SUCCESS {
		return out[:outSize-availOut]
	} else {
		return nil
	}
}
