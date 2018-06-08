package packet

import "errors"

const (
	//最大packet的字节数
	MAX_PACKET_BYTES = 2 * 1024 * 1024
	PACKET_HEAD_LEN  = (4 + 1 + 2 + 8 + 4) //请求头部长度	 int32
)

//包体过大
var ERR_TOO_LARGE_PACKET = errors.New("Too Large Packet")
