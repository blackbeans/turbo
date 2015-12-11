package codec

import (
	"bufio"
	"bytes"
	b "encoding/binary"
	"errors"
)

type IDecoder interface {
	//读取数据
	Read(reader *bufio.Reader) (*bytes.Buffer, error)
}

type LengthBasedFrameDecoder struct {
	IDecoder
	MaxFrameLength int32 //最大的包大小
	SkipLength     int16 //跳过长度字节数
}

//读取规定长度的数据
func (self LengthBasedFrameDecoder) Read(reader *bufio.Reader) (*bytes.Buffer, error) {
	var length int32

	err := Read(reader, b.BigEndian, &length)
	if nil != err {
		return nil, err
	} else if length <= 0 {
		return nil, errors.New("TOO SHORT PACKET")
	}

	if length > self.MaxFrameLength {
		return nil, errors.New("TOO LARGE PACKET!")
	}

	buff := make([]byte, int(length))
	l, err := reader.Read(buff)
	if nil != err {
		return nil, err
	}

	if l < int(length) {
		return nil, errors.New("ILLEGAL DATA PACKET!")
	}
	return bytes.NewBuffer(buff), nil
}
