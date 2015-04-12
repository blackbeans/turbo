package packet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

//请求的packet
type Packet struct {
	Opaque  int32
	CmdType uint8 //类型
	Data    []byte
}

func NewPacket(cmdtype uint8, data []byte) *Packet {
	return &Packet{
		Opaque:  -1,
		CmdType: cmdtype,
		Data:    data}
}

func (self *Packet) Reset() {
	self.Opaque = -1
}

func NewRespPacket(opaque int32, cmdtype uint8, data []byte) *Packet {
	p := NewPacket(cmdtype, data)
	p.Opaque = opaque
	return p
}

func (self *Packet) marshal() []byte {
	//总长度	4 字节+ 1字节 + 4字节 + var + \r + \n
	dl := 0
	if nil != self.Data {
		dl = len(self.Data)
	}
	length := PACKET_HEAD_LEN + dl + 2
	buffer := make([]byte, length)

	binary.BigEndian.PutUint32(buffer[0:4], uint32(self.Opaque))    // 请求id
	buffer[4] = self.CmdType                                        //数据类型
	binary.BigEndian.PutUint32(buffer[5:9], uint32(len(self.Data))) //总数据包长度
	copy(buffer[9:9+dl], self.Data)
	copy(buffer[9+dl:], CMD_CRLF)

	return buffer
}

var ERROR_PACKET_TYPE = errors.New("unmatches packet type ")

func (self *Packet) unmarshal(b []byte) error {

	if len(b) < PACKET_HEAD_LEN {
		return errors.New(fmt.Sprintf("Corrupt PacketData|Less Than MIN LENGTH:%d/%d", len(b), PACKET_HEAD_LEN))
	}

	self.Opaque = int32(binary.BigEndian.Uint32(b[:4]))

	self.CmdType = b[4]

	dataLength := binary.BigEndian.Uint32(b[5:9])

	if dataLength > 0 {
		if int(dataLength) == len(b[9:]) && dataLength <= MAX_PACKET_BYTES {
			//读取数据包
			self.Data = make([]byte, dataLength)
			copy(self.Data, b[9:])
		} else {
			if dataLength > MAX_PACKET_BYTES {
				return errors.New(fmt.Sprintf("Too Large Packet %d|%d", dataLength, MAX_PACKET_BYTES))
			}
			return errors.New("Corrupt PacketData ")
		}
	} else {
		return errors.New("Unmarshal|NO Data")
	}

	return nil
}

func MarshalPacket(packet *Packet) []byte {
	return packet.marshal()
}

//解码packet
func UnmarshalTLV(packet []byte) (*Packet, error) {
	packet = bytes.TrimSuffix(packet, CMD_CRLF)

	tlv := &Packet{}
	err := tlv.unmarshal(packet)
	if nil != err {
		return tlv, err
	} else {
		return tlv, nil
	}
}
