package msgpack

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"time"

	"github.com/vmihailenco/msgpack/v4/codes"
)

const (
	looseIfaceFlag uint32 = 1 << iota
	decodeUsingJSONFlag
	disallowUnknownFieldsFlag
)

const (
	bytesAllocLimit = 1e6 // 1mb
	sliceAllocLimit = 1e4
	maxMapSize      = 1e6
)

type bufReader interface {
	io.Reader
	io.ByteScanner
}

//------------------------------------------------------------------------------

var decPool = sync.Pool{
	New: func() interface{} {
		return NewDecoder(nil)
	},
}

// Unmarshal decodes the MessagePack-encoded data and stores the result
// in the value pointed to by v.
func Unmarshal(data []byte, v interface{}) error {
	dec := decPool.Get().(*Decoder)

	if r, ok := dec.r.(*bytes.Reader); ok {
		r.Reset(data)
	} else {
		dec.Reset(bytes.NewReader(data))
	}
	err := dec.Decode(v)

	decPool.Put(dec)

	return err
}

// A Decoder reads and decodes MessagePack values from an input stream.
type Decoder struct {
	r   io.Reader
	s   io.ByteScanner
	buf []byte

	extLen int
	rec    []byte // accumulates read data if not nil

	intern        []string
	flags         uint32
	decodeMapFunc func(*Decoder) (interface{}, error)
}

// NewDecoder returns a new decoder that reads from r.
//
// The decoder introduces its own buffering and may read data from r
// beyond the MessagePack values requested. Buffering can be disabled
// by passing a reader that implements io.ByteScanner interface.
func NewDecoder(r io.Reader) *Decoder {
	d := new(Decoder)
	d.Reset(r)
	return d
}

// Reset discards any buffered data, resets all state, and switches the buffered
// reader to read from r.
func (d *Decoder) Reset(r io.Reader) {
	if br, ok := r.(bufReader); ok {
		d.r = br
		d.s = br
	} else if br, ok := d.r.(*bufio.Reader); ok {
		br.Reset(r)
	} else {
		br := bufio.NewReader(r)
		d.r = br
		d.s = br
	}

	if d.intern != nil {
		d.intern = d.intern[:0]
	}

	//TODO:
	//d.useLoose = false
	//d.useJSONTag = false
	//d.disallowUnknownFields = false
	//d.decodeMapFunc = nil
}

func (d *Decoder) SetDecodeMapFunc(fn func(*Decoder) (interface{}, error)) {
	d.decodeMapFunc = fn
}

// UseDecodeInterfaceLoose causes decoder to use DecodeInterfaceLoose
// to decode msgpack value into Go interface{}.
func (d *Decoder) UseDecodeInterfaceLoose(on bool) *Decoder {
	if on {
		d.flags |= looseIfaceFlag
	} else {
		d.flags &= ^looseIfaceFlag
	}
	return d
}

// UseJSONTag causes the Decoder to use json struct tag as fallback option
// if there is no msgpack tag.
func (d *Decoder) UseJSONTag(on bool) *Decoder {
	if on {
		d.flags |= decodeUsingJSONFlag
	} else {
		d.flags &= ^decodeUsingJSONFlag
	}
	return d
}

// DisallowUnknownFields causes the Decoder to return an error when the destination
// is a struct and the input contains object keys which do not match any
// non-ignored, exported fields in the destination.
func (d *Decoder) DisallowUnknownFields() {
	if true {
		d.flags |= disallowUnknownFieldsFlag
	} else {
		d.flags &= ^disallowUnknownFieldsFlag
	}
}

// Buffered returns a reader of the data remaining in the Decoder's buffer.
// The reader is valid until the next call to Decode.
func (d *Decoder) Buffered() io.Reader {
	return d.r
}

//nolint:gocyclo
func (d *Decoder) Decode(v interface{}) error {
	var err error
	switch v := v.(type) {
	case *string:
		if v != nil {
			*v, err = d.DecodeString()
			return err
		}
	case *[]byte:
		if v != nil {
			return d.decodeBytesPtr(v)
		}
	case *int:
		if v != nil {
			*v, err = d.DecodeInt()
			return err
		}
	case *int8:
		if v != nil {
			*v, err = d.DecodeInt8()
			return err
		}
	case *int16:
		if v != nil {
			*v, err = d.DecodeInt16()
			return err
		}
	case *int32:
		if v != nil {
			*v, err = d.DecodeInt32()
			return err
		}
	case *int64:
		if v != nil {
			*v, err = d.DecodeInt64()
			return err
		}
	case *uint:
		if v != nil {
			*v, err = d.DecodeUint()
			return err
		}
	case *uint8:
		if v != nil {
			*v, err = d.DecodeUint8()
			return err
		}
	case *uint16:
		if v != nil {
			*v, err = d.DecodeUint16()
			return err
		}
	case *uint32:
		if v != nil {
			*v, err = d.DecodeUint32()
			return err
		}
	case *uint64:
		if v != nil {
			*v, err = d.DecodeUint64()
			return err
		}
	case *bool:
		if v != nil {
			*v, err = d.DecodeBool()
			return err
		}
	case *float32:
		if v != nil {
			*v, err = d.DecodeFloat32()
			return err
		}
	case *float64:
		if v != nil {
			*v, err = d.DecodeFloat64()
			return err
		}
	case *[]string:
		return d.decodeStringSlicePtr(v)
	case *map[string]string:
		return d.decodeMapStringStringPtr(v)
	case *map[string]interface{}:
		return d.decodeMapStringInterfacePtr(v)
	case *time.Duration:
		if v != nil {
			vv, err := d.DecodeInt64()
			*v = time.Duration(vv)
			return err
		}
	case *time.Time:
		if v != nil {
			*v, err = d.DecodeTime()
			return err
		}
	}

	vv := reflect.ValueOf(v)
	if !vv.IsValid() {
		return errors.New("msgpack: Decode(nil)")
	}
	if vv.Kind() != reflect.Ptr {
		return fmt.Errorf("msgpack: Decode(nonsettable %T)", v)
	}
	vv = vv.Elem()
	if !vv.IsValid() {
		return fmt.Errorf("msgpack: Decode(nonsettable %T)", v)
	}
	return d.DecodeValue(vv)
}

func (d *Decoder) DecodeMulti(v ...interface{}) error {
	for _, vv := range v {
		if err := d.Decode(vv); err != nil {
			return err
		}
	}
	return nil
}

func (d *Decoder) decodeInterfaceCond() (interface{}, error) {
	if d.flags&looseIfaceFlag != 0 {
		return d.DecodeInterfaceLoose()
	}
	return d.DecodeInterface()
}

func (d *Decoder) DecodeValue(v reflect.Value) error {
	decode := getDecoder(v.Type())
	return decode(d, v)
}

func (d *Decoder) DecodeNil() error {
	c, err := d.readCode()
	if err != nil {
		return err
	}
	if c != codes.Nil {
		return fmt.Errorf("msgpack: invalid code=%x decoding nil", c)
	}
	return nil
}

func (d *Decoder) decodeNilValue(v reflect.Value) error {
	err := d.DecodeNil()
	if v.IsNil() {
		return err
	}
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	v.Set(reflect.Zero(v.Type()))
	return err
}

func (d *Decoder) DecodeBool() (bool, error) {
	c, err := d.readCode()
	if err != nil {
		return false, err
	}
	return d.bool(c)
}

func (d *Decoder) bool(c codes.Code) (bool, error) {
	if c == codes.False {
		return false, nil
	}
	if c == codes.True {
		return true, nil
	}
	return false, fmt.Errorf("msgpack: invalid code=%x decoding bool", c)
}

func (d *Decoder) DecodeDuration() (time.Duration, error) {
	n, err := d.DecodeInt64()
	if err != nil {
		return 0, err
	}
	return time.Duration(n), nil
}

// DecodeInterface decodes value into interface. It returns following types:
//   - nil,
//   - bool,
//   - int8, int16, int32, int64,
//   - uint8, uint16, uint32, uint64,
//   - float32 and float64,
//   - string,
//   - []byte,
//   - slices of any of the above,
//   - maps of any of the above.
//
// DecodeInterface should be used only when you don't know the type of value
// you are decoding. For example, if you are decoding number it is better to use
// DecodeInt64 for negative numbers and DecodeUint64 for positive numbers.
func (d *Decoder) DecodeInterface() (interface{}, error) {
	c, err := d.readCode()
	if err != nil {
		return nil, err
	}

	if codes.IsFixedNum(c) {
		return int8(c), nil
	}
	if codes.IsFixedMap(c) {
		err = d.s.UnreadByte()
		if err != nil {
			return nil, err
		}
		return d.DecodeMap()
	}
	if codes.IsFixedArray(c) {
		return d.decodeSlice(c)
	}
	if codes.IsFixedString(c) {
		return d.string(c)
	}

	switch c {
	case codes.Nil:
		return nil, nil
	case codes.False, codes.True:
		return d.bool(c)
	case codes.Float:
		return d.float32(c)
	case codes.Double:
		return d.float64(c)
	case codes.Uint8:
		return d.uint8()
	case codes.Uint16:
		return d.uint16()
	case codes.Uint32:
		return d.uint32()
	case codes.Uint64:
		return d.uint64()
	case codes.Int8:
		return d.int8()
	case codes.Int16:
		return d.int16()
	case codes.Int32:
		return d.int32()
	case codes.Int64:
		return d.int64()
	case codes.Bin8, codes.Bin16, codes.Bin32:
		return d.bytes(c, nil)
	case codes.Str8, codes.Str16, codes.Str32:
		return d.string(c)
	case codes.Array16, codes.Array32:
		return d.decodeSlice(c)
	case codes.Map16, codes.Map32:
		err = d.s.UnreadByte()
		if err != nil {
			return nil, err
		}
		return d.DecodeMap()
	case codes.FixExt1, codes.FixExt2, codes.FixExt4, codes.FixExt8, codes.FixExt16,
		codes.Ext8, codes.Ext16, codes.Ext32:
		return d.extInterface(c)
	}

	return 0, fmt.Errorf("msgpack: unknown code %x decoding interface{}", c)
}

// DecodeInterfaceLoose is like DecodeInterface except that:
//   - int8, int16, and int32 are converted to int64,
//   - uint8, uint16, and uint32 are converted to uint64,
//   - float32 is converted to float64.
func (d *Decoder) DecodeInterfaceLoose() (interface{}, error) {
	c, err := d.readCode()
	if err != nil {
		return nil, err
	}

	if codes.IsFixedNum(c) {
		return int64(int8(c)), nil
	}
	if codes.IsFixedMap(c) {
		err = d.s.UnreadByte()
		if err != nil {
			return nil, err
		}
		return d.DecodeMap()
	}
	if codes.IsFixedArray(c) {
		return d.decodeSlice(c)
	}
	if codes.IsFixedString(c) {
		return d.string(c)
	}

	switch c {
	case codes.Nil:
		return nil, nil
	case codes.False, codes.True:
		return d.bool(c)
	case codes.Float, codes.Double:
		return d.float64(c)
	case codes.Uint8, codes.Uint16, codes.Uint32, codes.Uint64:
		return d.uint(c)
	case codes.Int8, codes.Int16, codes.Int32, codes.Int64:
		return d.int(c)
	case codes.Bin8, codes.Bin16, codes.Bin32:
		return d.bytes(c, nil)
	case codes.Str8, codes.Str16, codes.Str32:
		return d.string(c)
	case codes.Array16, codes.Array32:
		return d.decodeSlice(c)
	case codes.Map16, codes.Map32:
		err = d.s.UnreadByte()
		if err != nil {
			return nil, err
		}
		return d.DecodeMap()
	case codes.FixExt1, codes.FixExt2, codes.FixExt4, codes.FixExt8, codes.FixExt16,
		codes.Ext8, codes.Ext16, codes.Ext32:
		return d.extInterface(c)
	}

	return 0, fmt.Errorf("msgpack: unknown code %x decoding interface{}", c)
}

// Skip skips next value.
func (d *Decoder) Skip() error {
	c, err := d.readCode()
	if err != nil {
		return err
	}

	if codes.IsFixedNum(c) {
		return nil
	}
	if codes.IsFixedMap(c) {
		return d.skipMap(c)
	}
	if codes.IsFixedArray(c) {
		return d.skipSlice(c)
	}
	if codes.IsFixedString(c) {
		return d.skipBytes(c)
	}

	switch c {
	case codes.Nil, codes.False, codes.True:
		return nil
	case codes.Uint8, codes.Int8:
		return d.skipN(1)
	case codes.Uint16, codes.Int16:
		return d.skipN(2)
	case codes.Uint32, codes.Int32, codes.Float:
		return d.skipN(4)
	case codes.Uint64, codes.Int64, codes.Double:
		return d.skipN(8)
	case codes.Bin8, codes.Bin16, codes.Bin32:
		return d.skipBytes(c)
	case codes.Str8, codes.Str16, codes.Str32:
		return d.skipBytes(c)
	case codes.Array16, codes.Array32:
		return d.skipSlice(c)
	case codes.Map16, codes.Map32:
		return d.skipMap(c)
	case codes.FixExt1, codes.FixExt2, codes.FixExt4, codes.FixExt8, codes.FixExt16,
		codes.Ext8, codes.Ext16, codes.Ext32:
		return d.skipExt(c)
	}

	return fmt.Errorf("msgpack: unknown code %x", c)
}

// PeekCode returns the next MessagePack code without advancing the reader.
// Subpackage msgpack/codes contains list of available codes.
func (d *Decoder) PeekCode() (codes.Code, error) {
	c, err := d.s.ReadByte()
	if err != nil {
		return 0, err
	}
	return codes.Code(c), d.s.UnreadByte()
}

func (d *Decoder) hasNilCode() bool {
	code, err := d.PeekCode()
	return err == nil && code == codes.Nil
}

func (d *Decoder) readCode() (codes.Code, error) {
	d.extLen = 0
	c, err := d.s.ReadByte()
	if err != nil {
		return 0, err
	}
	if d.rec != nil {
		d.rec = append(d.rec, c)
	}
	return codes.Code(c), nil
}

func (d *Decoder) readFull(b []byte) error {
	_, err := io.ReadFull(d.r, b)
	if err != nil {
		return err
	}
	if d.rec != nil {
		//TODO: read directly into d.rec?
		d.rec = append(d.rec, b...)
	}
	return nil
}

func (d *Decoder) readN(n int) ([]byte, error) {
	var err error
	d.buf, err = readN(d.r, d.buf, n)
	if err != nil {
		return nil, err
	}
	if d.rec != nil {
		//TODO: read directly into d.rec?
		d.rec = append(d.rec, d.buf...)
	}
	return d.buf, nil
}

func readN(r io.Reader, b []byte, n int) ([]byte, error) {
	if b == nil {
		if n == 0 {
			return make([]byte, 0), nil
		}
		switch {
		case n < 64:
			b = make([]byte, 0, 64)
		case n <= bytesAllocLimit:
			b = make([]byte, 0, n)
		default:
			b = make([]byte, 0, bytesAllocLimit)
		}
	}

	if n <= cap(b) {
		b = b[:n]
		_, err := io.ReadFull(r, b)
		return b, err
	}
	b = b[:cap(b)]

	var pos int
	for {
		alloc := min(n-len(b), bytesAllocLimit)
		b = append(b, make([]byte, alloc)...)

		_, err := io.ReadFull(r, b[pos:])
		if err != nil {
			return b, err
		}

		if len(b) == n {
			break
		}
		pos = len(b)
	}

	return b, nil
}

func min(a, b int) int { //nolint:unparam
	if a <= b {
		return a
	}
	return b
}
