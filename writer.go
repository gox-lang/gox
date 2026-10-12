package gox

import (
	"io"
	"math/big"
)

// Writer is the output contract used by compiled Gox plugins.
// The implementation is provided by gox-server.
type Writer interface {
	io.Writer

	WriteTextString(value string)
	WriteRawString(value string)
	WriteTextBytes(value []byte)
	WriteRawBytes(value []byte)
	WriteBytes(value []byte)
	// WriteByte writes one byte and reports failures to the caller.
	WriteByte(value byte) error
	WriteStream(value io.Reader)
	WriteError(value error)
	WriteTextBool(value bool)
	WriteRawBool(value bool)
	WriteTextInt(value int)
	WriteRawInt(value int)
	WriteTextInt8(value int8)
	WriteRawInt8(value int8)
	WriteTextInt16(value int16)
	WriteRawInt16(value int16)
	WriteTextInt32(value int32)
	WriteRawInt32(value int32)
	WriteTextInt64(value int64)
	WriteRawInt64(value int64)
	WriteTextUint(value uint)
	WriteRawUint(value uint)
	WriteTextUint8(value uint8)
	WriteRawUint8(value uint8)
	WriteTextUint16(value uint16)
	WriteRawUint16(value uint16)
	WriteTextUint32(value uint32)
	WriteRawUint32(value uint32)
	WriteTextUint64(value uint64)
	WriteRawUint64(value uint64)
	WriteTextUintptr(value uintptr)
	WriteRawUintptr(value uintptr)
	WriteTextFloat32(value float32)
	WriteRawFloat32(value float32)
	WriteTextFloat64(value float64)
	WriteRawFloat64(value float64)
	WriteTextBigInt(value *big.Int)
	WriteRawBigInt(value *big.Int)
	WriteTextBigIntValue(value big.Int)
	WriteRawBigIntValue(value big.Int)
	WriteTextBigFloat(value *big.Float)
	WriteRawBigFloat(value *big.Float)
	WriteTextBigFloatValue(value big.Float)
	WriteRawBigFloatValue(value big.Float)
	WriteTextBigRat(value *big.Rat)
	WriteRawBigRat(value *big.Rat)
	WriteTextBigRatValue(value big.Rat)
	WriteRawBigRatValue(value big.Rat)
	WriteTextAny(value any)
	WriteRawAny(value any)
	WriteSpreadAttrs(attrs map[string]string)
}
