package contract

import (
	"encoding/binary"
	"fmt"
)

type wireField struct {
	Number int
	Type   int
	Varint uint64
	Bytes  []byte
}

func scanWire(data []byte, fn func(wireField) error) error {
	for len(data) > 0 {
		key, n := consumeVarint(data)
		if n <= 0 {
			return fmt.Errorf("contract: malformed protobuf field key")
		}
		data = data[n:]
		number := int(key >> 3)
		wireType := int(key & 7)
		if number <= 0 {
			return fmt.Errorf("contract: invalid protobuf field number %d", number)
		}
		field := wireField{Number: number, Type: wireType}
		switch wireType {
		case 0:
			value, consumed := consumeVarint(data)
			if consumed <= 0 {
				return fmt.Errorf("contract: malformed varint field %d", number)
			}
			field.Varint = value
			data = data[consumed:]
		case 1:
			if len(data) < 8 {
				return fmt.Errorf("contract: malformed fixed64 field %d", number)
			}
			field.Bytes = data[:8]
			data = data[8:]
		case 2:
			length, consumed := consumeVarint(data)
			if consumed <= 0 || length > uint64(len(data)-consumed) {
				return fmt.Errorf("contract: malformed bytes field %d", number)
			}
			data = data[consumed:]
			field.Bytes = data[:int(length)]
			data = data[int(length):]
		case 5:
			if len(data) < 4 {
				return fmt.Errorf("contract: malformed fixed32 field %d", number)
			}
			field.Bytes = data[:4]
			data = data[4:]
		default:
			return fmt.Errorf("contract: unsupported protobuf wire type %d", wireType)
		}
		if err := fn(field); err != nil {
			return err
		}
	}
	return nil
}

func consumeVarint(data []byte) (uint64, int) {
	var value uint64
	for i := 0; i < len(data) && i < binary.MaxVarintLen64; i++ {
		b := data[i]
		if b < 0x80 {
			if i == 9 && b > 1 {
				return 0, -1
			}
			return value | uint64(b)<<uint(7*i), i + 1
		}
		value |= uint64(b&0x7f) << uint(7*i)
	}
	return 0, -1
}

func packedInt32(data []byte) ([]int32, error) {
	values := make([]int32, 0)
	for len(data) > 0 {
		value, n := consumeVarint(data)
		if n <= 0 {
			return nil, fmt.Errorf("contract: malformed packed int32")
		}
		values = append(values, int32(value))
		data = data[n:]
	}
	return values, nil
}
