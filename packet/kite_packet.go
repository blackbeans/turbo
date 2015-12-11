package packet

import (
	"bytes"
	"errors"
	"fmt"
)

//请求的packet
type Packet struct {
	Header *PacketHeader
	Data   []byte
}

func NewPacket(cmdtype uint8, data []byte) *Packet {
	h := &PacketHeader{Opaque: -1, CmdType: cmdtype}
	return &Packet{Header: h,
		Data: data}
}

func (self *Packet) Reset() {
	self.Header.Opaque = -1
}

func NewRespPacket(opaque int32, cmdtype uint8, data []byte) *Packet {
	p := NewPacket(cmdtype, data)
	p.Header.Opaque = opaque
	return p
}

func (self *Packet) marshal() []byte {
	dl := 0
	if nil != self.Data {
		dl = len(self.Data)
	}

	buff := MarshalHeader(self.Header, int32(dl))
	buff.Write(self.Data)
	return buff.Bytes()
}

var ERROR_PACKET_TYPE = errors.New("unmatches packet type ")

func MarshalPacket(packet *Packet) []byte {
	return packet.marshal()
}

func UnmarshalTLV(buff *bytes.Buffer) (*Packet, error) {

	tlv := &Packet{}
	if buff.Len() < PACKET_HEAD_LEN {
		return nil, errors.New(
			fmt.Sprintf("Corrupt PacketData|Less Than MIN LENGTH:%d/%d", buff.Len(), PACKET_HEAD_LEN))
	}
	header, err := UnmarshalHeader(buff)
	if nil != err {
		return nil, errors.New(
			fmt.Sprintf("Corrupt PacketHeader|%s", err.Error()))
	}

	if header.BodyLen > MAX_PACKET_BYTES {
		return nil, errors.New(fmt.Sprintf("Too Large Packet %d|%d", header.BodyLen, MAX_PACKET_BYTES))
	}
	tlv.Header = header
	tlv.Data = buff.Bytes()

	return tlv, nil
}
