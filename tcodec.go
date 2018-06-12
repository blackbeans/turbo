package turbo

import "errors"

const (
	//最大packet的字节数
	MAX_PACKET_BYTES = 2 * 1024 * 1024
	PACKET_HEAD_LEN  = (4 + 1 + 2 + 8 + 4) //请求头部长度	 int32
)

//包体过大
var ERR_TOO_LARGE_PACKET = errors.New("Too Large Packet")
var ERR_TIMEOUT = errors.New("WAIT RESPONSE TIMEOUT ")
var ERR_MARSHAL_PACKET = errors.New("ERROR MARSHAL PACKET")
var ERR_OVER_FLOW = errors.New("Group Over Flow")
var ERR_NO_HOSTS = errors.New("NO VALID RemoteClient")
var ERR_PONG = errors.New("ERROR PONG TYPE !")


type ICodec interface {

	//包装秤packet
	UnmarshalPayload(p Packet) (Packet, error)
	//序列化packet
	MarshalPayload(p Packet) ([]byte, error)
}

type LengthBasedCodec struct {
	ICodec
	MaxFrameLength int32 //最大的包大小
	SkipLength     int16 //跳过长度字节数
}

//反序列化
func (self LengthBasedCodec) UnmarshalPacket(p Packet) (Packet, error) {
	return p, nil
}

//序列化
func (self LengthBasedCodec) MarshalPacket(packet Packet) ([]byte, error) {
	return packet.Data, nil
}
