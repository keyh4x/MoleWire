package wire

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const headerSize = 4
const MaxSize = 64 << 10 // 64 kb
var SizeTooLarge = errors.New("wire: frame larger than MaxSize")

func WriteFrame(w io.Writer, payload []byte)error{
	if len(payload) > MaxSize{
		return SizeTooLarge
	}
	buf := make([]byte, headerSize+len(payload))
	binary.BigEndian.PutUint32(buf[:headerSize], uint32(len(payload)))
	copy(buf[headerSize:], payload)

	_, err := w.Write(buf)
	if err != nil{
		return fmt.Errorf("wire: write frame: %w", err)
	}
	return nil
}

func ReadFrame(r io.Reader)([]byte, error){
	var hdr [headerSize]byte
	_, err := io.ReadFull(r, hdr[:])
	if err != nil{
		return nil, err
	}

	size := binary.BigEndian.Uint32(hdr[:])
	if size > MaxSize{
		return nil, SizeTooLarge
	}

	payload := make([]byte, size)
	_, err = io.ReadFull(r, payload)
	if err != nil{
		return nil, fmt.Errorf("wire: read payload: %w", err)
	}
	return payload, nil
}
