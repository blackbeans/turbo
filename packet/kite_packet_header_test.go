package packet

import (
	"bytes"
	"fmt"
	"testing"
)

func TestHeader(t *testing.T) {
	p := NewPacket(1, []byte("echo"))
	fmt.Printf("%t\n", p)
	buff := MarshalPacket(p)
	fmt.Printf("%t\n", buff)
	buf := bytes.NewBuffer(buff[4:])
	fmt.Printf("%t\n", buff)

	packet, _ := UnmarshalTLV(buf)
	fmt.Printf("%t\n", packet)
}
