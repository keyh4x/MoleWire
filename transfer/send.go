package transfer

import (
	"MoleWire/message"
	"MoleWire/wire"
	"errors"
	"fmt"
	"io"
	"os"
	"sync/atomic"
)

type Sender struct{
	conn         io.ReadWriter
	file         *os.File
	hash         [32]byte
	fileSize     uint64
	chunkSize    uint32
	name         string
	MoleAccepted bool 
	sentBytes    atomic.Uint64
}

var noBurrow = errors.New("peer sent a request before burrowing")

func (s *Sender)Serve()error{
	var stop func()
	defer func(){
		if stop != nil{
			stop()
		}
	}()
	for{
		body, err := wire.ReadFrame(s.conn)
		if err == io.EOF{
			return nil // person closed in between
		}
		if err != nil{
			return err
		}
		if len(body) == 0{ // keep alive check
			continue
		}
		msg, err := message.Decode(body)
		if err != nil{
			s.sendCollapse(message.CodeMalformed, "cant read that, its garbage")
			return err
		}
		switch m := msg.(type){
		case message.Burrow:
			accepted, err := s.handleBurrow(m)
			if err != nil{
				return err
			}
			if !accepted{
				return nil 
			}
			if stop == nil{
				stop = startProgress("Filling the tunnel...","", s.fileSize, &s.sentBytes)
			}
		case message.Dig:
			if !s.MoleAccepted{
				s.sendCollapse(message.CodeMalformed, "burrow first :D")
				return noBurrow
			}
			err = s.sendDirt(m.Index)
			if err != nil{
				return err
			}
		case message.Bury:
			return nil // file transfer done
		default:
			s.sendCollapse(message.CodeMalformed, "wasnt expecting that one")
			return fmt.Errorf("unexpected: %T", msg)
		}
	}
}

func (s *Sender)chunkCount()uint64{
	if s.chunkSize == 0{
		return 0
	}
	return (s.fileSize + uint64(s.chunkSize) - 1) / uint64(s.chunkSize)
}

func (s *Sender)handleBurrow(b message.Burrow)(accepted bool, err error){
	if b.Version != message.Version{
		return false, s.sendCollapse(message.CodeBadVersion, "we speak different molewire versions")
	}
	if b.FileHash != s.hash{
		return false, s.sendCollapse(message.CodeUnknownFile, "wrong burrow")
	}
	sc := message.Scent{
		FileSize:  s.fileSize,
		ChunkSize: s.chunkSize,
		Name:      s.name,
	}
	body, err := sc.Encode()
	if err != nil{
		return false, err
	}
	err = wire.WriteFrame(s.conn, body)
	if err != nil{
		return false, err
	}
	s.MoleAccepted = true
	return true, nil
}

func (s *Sender)sendCollapse(code message.CollapseCode, reason string)error{
	c := message.Collapse{
		Code:   code,
		Reason: reason,
	}
	body, err := c.Encode()
	if err != nil{
		return err
	}
	return wire.WriteFrame(s.conn, body)
}

func (s *Sender)sendDirt(index uint32)error{
	if uint64(index) >= s.chunkCount(){
		return s.sendCollapse(message.CodeOutOfRange, "got a chunk out of index")
	}
	offset := int64(index) * int64(s.chunkSize)
	buf := make([]byte, s.chunkSize)

	n, err := s.file.ReadAt(buf, offset)
	if err != nil && err != io.EOF{
		return err
	}
	d := message.Dirt{
		Index: index,
		Data:  buf[:n], 
	}
	body, err := d.Encode()
	if err != nil{
		return err
	}
	err = wire.WriteFrame(s.conn, body)
	if err != nil{
		return err
	}
	s.sentBytes.Add(uint64(n)) // only the chunk
	return nil
}

func (s *Sender)FileHash()([32]byte, error){
	return HashFile(s.file)
}
