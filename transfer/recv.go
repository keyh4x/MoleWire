package transfer

import (
	"MoleWire/message"
	"MoleWire/wire"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

type Receiver struct{
	conn         io.ReadWriter
	downloadPath string
	wanthash     [32]byte
	fileSize     uint64
	chunkSize    uint32
	name         string
}

func (r *Receiver)chunkCount()uint64{
	if r.chunkSize == 0{
		return 0
	}
	return (r.fileSize + uint64(r.chunkSize) - 1) / uint64(r.chunkSize) // ceil
}

func NewReceiver(Conn io.ReadWriter, wantHash [32]byte, downloadPath string)*Receiver{
	return &Receiver{
		conn:         Conn,
		downloadPath: downloadPath,
		wanthash:     wantHash,
	}
}

func removeTraversal(filename string)(string, error){
	if filename == ""{
		return "", errors.New("empty filename")
	}
	name := filepath.Base(filename)
	if strings.ContainsAny(name, `/\`){
		return "", errors.New("invalid filename")
	}
	if name == "." || name == ".."{
		return "", errors.New("invalid filename")
	}
	return name, nil
}

func (r *Receiver)createFile()(*os.File, string, error){
	name, err := removeTraversal(r.name)
	if err != nil{
		return nil, "", err
	}

	err = os.MkdirAll(r.downloadPath, 0o755)
	if err != nil{
		return nil, "", err
	}

	finalPath := filepath.Join(r.downloadPath, name)
	file, err := os.Create(finalPath + ".partial")
	if err != nil{
		return nil, "", err
	}
	return file, finalPath, nil
}

func (r *Receiver)readReply()(message.Message, error){
	for{
		body, err := wire.ReadFrame(r.conn)
		if err != nil{
			return nil, err
		}
		if len(body) == 0{ // keep alive check
			continue
		}
		return message.Decode(body)
	}
}

func (r *Receiver)Fetch()error{
	err := r.handshake()
	if err != nil{
		return err
	}

	file, finalPath, err := r.createFile()
	if err != nil{
		return err
	}
	partialPath := file.Name()

	err = r.downloadAndVerify(file)

	closeErr := file.Close()

	if err != nil{
		os.Remove(partialPath)
		return err
	}
	if closeErr != nil{
		os.Remove(partialPath)
		return closeErr
	}

	err = os.Rename(partialPath, finalPath)
	if err != nil{
		return err
	}

	return r.sendBury()
}

const maxDigs = 128
func (r *Receiver)download(file *os.File)error{
	if r.chunkSize == 0{
		return errors.New("peer sent a zero chunk size")
	}
	totalChunks := r.chunkCount() 

	if totalChunks > math.MaxUint32{
		return fmt.Errorf("file needs %d chunks, protocol allows %d", totalChunks, uint64(math.MaxUint32))
	}
	nextDig := uint64(0)                  // next chunk
	gotDirt := uint64(0)                  // current chunk
	openDigs := uint64(0)                 // req sent for waiting response
	received := make([]bool, totalChunks) // received chunks
	var bytesGot atomic.Uint64
	stop := startProgress("Digging...", "file downloaded successfully", r.fileSize, &bytesGot)
	defer stop()

	for gotDirt < totalChunks{
		for openDigs < maxDigs && nextDig < totalChunks{
			err := r.sendDig(uint32(nextDig))
			if err != nil{
				return err
			}
			nextDig++
			openDigs++
		}
		msg, err := r.readReply()
		if err != nil{
			return err
		}

		switch m := msg.(type){
		case message.Dirt:
			if uint64(m.Index) >= totalChunks{
				return fmt.Errorf("out of range index")
			}
			if uint64(m.Index) >= nextDig{
				return fmt.Errorf("unknown chunk was sent") 
			}
			if received[m.Index]{
				return fmt.Errorf("peer sent a duplicate chunk")
			}
			want := r.expectedChunkLen(m.Index, totalChunks)
			if uint64(len(m.Data)) != want{
				return fmt.Errorf("chunk %d: got: %d bytes want: %d", m.Index, len(m.Data), want)
			}
			offset := int64(m.Index) * int64(r.chunkSize)
			_, err = file.WriteAt(m.Data, offset)
			if err != nil{
				return err
			}
			received[m.Index] = true
			bytesGot.Add(uint64(len(m.Data)))
			gotDirt++
			openDigs--
		case message.Collapse:
			return fmt.Errorf("got refused: %s code : %d", m.Reason, m.Code)
		default:
			return fmt.Errorf("expected dirt...got %T", msg)
		}
	}
	return nil
}

func (r *Receiver)expectedChunkLen(index uint32, totalChunks uint64)uint64{
	if uint64(index) == totalChunks-1{
		return r.fileSize - uint64(index)*uint64(r.chunkSize) // last chunk of the file
	}
	return uint64(r.chunkSize)
}

func (r *Receiver)handshake()error{
	err := r.sendBurrow()
	if err != nil{
		return err
	}
	msg, err := r.readReply()
	if err != nil{
		return err
	}
	switch m := msg.(type){
	case message.Scent:
		r.fileSize = m.FileSize
		r.chunkSize = m.ChunkSize
		r.name = m.Name
		return nil
	case message.Collapse:
		return fmt.Errorf("got refused: %s code: %d", m.Reason, m.Code)
	default:
		return fmt.Errorf("expected scent...got  %T", msg)
	}
}

func (r *Receiver)sendBurrow()error{
	b := message.Burrow{
		Version:  message.Version,
		FileHash: r.wanthash,
	}
	body, err := b.Encode()
	if err != nil{
		return err
	}
	return wire.WriteFrame(r.conn, body)
}

func (r *Receiver)sendDig(index uint32)error{
	d := message.Dig{
		Index: index,
	}
	body, err := d.Encode()
	if err != nil{
		return err
	}
	return wire.WriteFrame(r.conn, body)
}

func (r *Receiver)sendBury()error{
	b := message.Bury{}
	body, err := b.Encode()
	if err != nil{
		return err
	}
	return wire.WriteFrame(r.conn, body)
}

func (r *Receiver)downloadAndVerify(file *os.File)error{
	err := r.download(file)
	if err != nil{
		return err
	}

	got, err := HashFile(file)
	if err != nil{
		return err
	}
	if got != r.wanthash{
		return fmt.Errorf("hash mismatch: got %x, want %x", got, r.wanthash)
	}
	return nil
}

