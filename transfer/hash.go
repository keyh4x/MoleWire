package transfer

import (
	"crypto/sha256"
	"io"
	"os"
)

func HashFile(f *os.File)([32]byte, error){
	var sum [32]byte
	_, err := f.Seek(0, io.SeekStart)
	if err != nil{
		return sum, err
	}
	hasher := sha256.New()
	_, err = io.Copy(hasher, f)
	if err != nil{
		return sum, err
	}

	copy(sum[:], hasher.Sum(nil))
	return sum, nil
}
