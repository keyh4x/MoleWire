package transfer

import (
	"os"
	"io"

)
type Offer struct{
	file *os.File
	Hash [32]byte
	Size uint64
	Name string
	ChunkSize uint32
}

const DefaultChunkSize = 16 << 10 //16 kb

func OpenFile(path string)(*Offer , error){
	f , err := os.Open(path)
	if err != nil{
		f.Close()
		return  nil , err
	}
	hash , err := HashFile(f)
	if err != nil{
		f.Close()
		return  nil , err
	}
	info , err := f.Stat()
	if err != nil{
		f.Close()
		return nil , err
	}
	
	name := info.Name()
	size := uint64(info.Size())

	return &Offer{
		file: f,
		Hash: hash,
		Size: size,
		Name: name,
		ChunkSize: DefaultChunkSize,
	} , nil

}
func (o *Offer)Sender(Conn io.ReadWriter)*Sender{
	return &Sender{
		conn: Conn,
		file: o.file,
		hash: o.Hash,
		fileSize: o.Size,
		chunkSize: o.ChunkSize,
		name: o.Name,
		MoleAccepted: false,
	}
}

func (o *Offer)Close()error{
	return o.file.Close()
}