package message

import (
	"bytes"
	"encoding/binary"
	"errors"
)

var EmptyBody = errors.New("message: empty body")
var UnknownType = errors.New("message: unknown type")
var Malformed = errors.New("message: wrong length")
var BadMagic = errors.New("message: not a MoleWire peer")
var NameTooLong = errors.New("message: filename too long")
var Badversion = errors.New("message: bad version")

type MsgType byte
const (
	TypeBurrow   MsgType = 0x01 // handshake
	TypeScent    MsgType = 0x02 // metadata(file)
	TypeDig      MsgType = 0x03 // requesting chunk
	TypeDirt     MsgType = 0x04 // chunk
	TypeBury     MsgType = 0x05 // successful
	TypeCollapse MsgType = 0x06 // rejection
)

var Mole = [4]byte{'M', 'O', 'L', 'E'}
const Version byte = 0x01
const NameLen = 255

type Message interface{
	Encode() ([]byte, error)
}

type Burrow struct{
	Version  byte
	FileHash [32]byte
}

type Scent struct{
	FileSize  uint64
	ChunkSize uint32
	Name      string
}

type Dig struct{
	Index uint32
}

type Dirt struct{
	Index uint32
	Data  []byte
}

type Bury struct{}

type CollapseCode byte 
const CodeUnknownFile CollapseCode = 0x01
const CodeBadVersion  CollapseCode = 0x02
const CodeOutOfRange  CollapseCode = 0x03
const CodeMalformed   CollapseCode = 0x04
const CodeInternal    CollapseCode = 0x05

type Collapse struct{
	Code   CollapseCode
	Reason string
}

func (d Dig)Encode()([]byte, error){
	buff := make([]byte, 5)
	buff[0] = byte(TypeDig)
	binary.BigEndian.PutUint32(buff[1:], d.Index)
	return buff, nil

}
func (b Bury)Encode()([]byte, error){
	return []byte{byte(TypeBury)}, nil
}

//(0x01 1 byte)(MOLE 4 bytes)(ver 1 byte)(hash 32 bytes) = 38
func (b Burrow)Encode()([]byte, error){
	buff := make([]byte, 38)
	buff[0] = byte(TypeBurrow)
	copy(buff[1:5], Mole[:])
	buff[5] = b.Version
	copy(buff[6:38], b.FileHash[:])
	return buff, nil
}

//(0x06 1 byte)(code 1 byte)(reason...)
func (c Collapse)Encode()([]byte, error){
	buff := make([]byte, 2+len(c.Reason))
	buff[0] = byte(TypeCollapse)
	buff[1] = byte(c.Code)
	copy(buff[2:], c.Reason)
	return buff, nil
}

//(0x04 1 byte)(index 4 bytes)(data...)
func (d Dirt)Encode()([]byte, error){
	buff := make([]byte, 5+len(d.Data))
	buff[0] = byte(TypeDirt)
	binary.BigEndian.PutUint32(buff[1:5], d.Index)
	copy(buff[5:], d.Data)
	return buff, nil
}

//(02 1 byte)(fileSize 8 bytes)(chunkSize 4 bytes)(nameLen 2 bytes)(name...) = 15 + name
func (s Scent)Encode()([]byte, error){ 
	if len(s.Name) > NameLen{
		return nil, NameTooLong
	}
	buff := make([]byte, 15+len(s.Name))
	buff[0] = byte(TypeScent)
	binary.BigEndian.PutUint64(buff[1:9], s.FileSize)
	binary.BigEndian.PutUint32(buff[9:13], s.ChunkSize)
	binary.BigEndian.PutUint16(buff[13:15], uint16(len(s.Name)))
	copy(buff[15:], s.Name)
	return buff, nil
}

func Decode(body []byte)(Message, error){
	if len(body) == 0{
		return nil, EmptyBody
	}
	switch MsgType(body[0]){
	case TypeBurrow:
		if len(body) != 38{
			return nil, Malformed
		}
		if !bytes.Equal(body[1:5], Mole[:]){
			return nil, BadMagic
		}
		b := Burrow{Version: body[5]}
		copy(b.FileHash[:], body[6:38])
		return b, nil
	case TypeScent:
		if len(body) < 15{
			return nil, Malformed
		}
		nameLen := int(binary.BigEndian.Uint16(body[13:15]))
		if nameLen > NameLen{
			return nil, NameTooLong
		}
		if len(body) != 15+nameLen{
			return nil, Malformed
		}
		return Scent{
			FileSize:  binary.BigEndian.Uint64(body[1:9]),
			ChunkSize: binary.BigEndian.Uint32(body[9:13]),
			Name:      string(body[15:]),
		}, nil
	case TypeDig:
		if len(body) != 5{
			return nil, Malformed
		}
		return Dig{Index: binary.BigEndian.Uint32(body[1:5])}, nil
	case TypeDirt:
		if len(body) < 5{
			return nil, Malformed
		}
		data := make([]byte, len(body)-5)
		copy(data, body[5:])
		return Dirt{
			Index: binary.BigEndian.Uint32(body[1:5]),
			Data:  data,
		}, nil
	case TypeBury:
		if len(body) != 1{
			return nil, Malformed
		}
		return Bury{}, nil

	case TypeCollapse:
		if len(body) < 2{
			return nil, Malformed
		}
		return Collapse{
			Code:   CollapseCode(body[1]),
			Reason: string(body[2:]),
		}, nil
	default:
		return nil, UnknownType
	}
}
