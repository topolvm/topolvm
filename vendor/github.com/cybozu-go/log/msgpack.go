package log

import (
	"encoding/binary"
	"math"
	"time"
)

// MessagePack type tags.
// Only tags used in this code are defined.
const (
	mpNil      = 0xc0
	mpFalse    = 0xc2
	mpTrue     = 0xc3
	mpInt16    = 0xd1
	mpInt32    = 0xd2
	mpInt64    = 0xd3
	mpFixStr   = 0xa0
	mpStr8     = 0xd9
	mpStr16    = 0xda
	mpStr32    = 0xdb
	mpBin8     = 0xc4
	mpBin16    = 0xc5
	mpBin32    = 0xc6
	mpFixArray = 0x90
	mpArray16  = 0xdc
	mpArray32  = 0xdd
	mpFixMap   = 0x80
	mpMap16    = 0xde
)

func appendMsgpackInt64(b []byte, n int64) []byte {
	switch {
	case 0 <= n && n <= 127:
		return append(b, byte(n))
	case math.MinInt16 <= n && n <= math.MaxInt16:
		b = append(b, mpInt16, 0, 0)
		binary.BigEndian.PutUint16(b[len(b)-2:], uint16(n))
		return b
	case math.MinInt32 <= n && n <= math.MaxInt32:
		b = append(b, mpInt32, 0, 0, 0, 0)
		binary.BigEndian.PutUint32(b[len(b)-4:], uint32(n))
		return b
	default:
		b = append(b, mpInt64, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(b[len(b)-8:], uint64(n))
		return b
	}
}

func appendMsgpackString(b []byte, s string) ([]byte, error) {
	switch {
	case len(s) >= maxLogSize:
		return nil, ErrTooLarge
	case len(s) <= 31:
		b = append(b, byte(mpFixStr+len(s)))
	case len(s) <= math.MaxUint8:
		b = append(b, byte(mpStr8))
		b = append(b, byte(len(s)))
	case len(s) <= math.MaxUint16:
		b = append(b, byte(mpStr16), 0, 0)
		binary.BigEndian.PutUint16(b[len(b)-2:], uint16(len(s)))
	case uint32(len(s)) <= math.MaxUint32:
		b = append(b, byte(mpStr32), 0, 0, 0, 0)
		binary.BigEndian.PutUint32(b[len(b)-4:], uint32(len(s)))
	}
	return append(b, s...), nil
}

func appendMsgpackArray(b []byte, length int) ([]byte, error) {
	switch {
	case length <= 15:
		return append(b, byte(mpFixArray+length)), nil
	case length <= math.MaxUint16:
		b = append(b, byte(mpArray16), 0, 0)
		binary.BigEndian.PutUint16(b[len(b)-2:], uint16(length))
		return b, nil
	case uint32(length) <= math.MaxUint32:
		b = append(b, byte(mpArray32), 0, 0, 0, 0)
		binary.BigEndian.PutUint32(b[len(b)-4:], uint32(length))
		return b, nil
	default:
		return nil, ErrTooLarge
	}
}

func appendMsgpack(b []byte, v interface{}) ([]byte, error) {
	switch t := v.(type) {
	case nil:
		return append(b, mpNil), nil
	case bool:
		if t {
			return append(b, mpTrue), nil
		}
		return append(b, mpFalse), nil
	case int:
		return appendMsgpackInt64(b, int64(t)), nil
	case int64:
		return appendMsgpackInt64(b, t), nil
	case time.Time:
		return appendMsgpackInt64(b, t.UnixNano()/1000), nil
	case string:
		return appendMsgpackString(b, t)
	case []byte:
		switch {
		case len(t) >= maxLogSize:
			return nil, ErrTooLarge
		case len(t) <= math.MaxUint8:
			b = append(b, byte(mpBin8))
			b = append(b, byte(len(t)))
		case len(t) <= math.MaxUint16:
			b = append(b, byte(mpBin16), 0, 0)
			binary.BigEndian.PutUint16(b[len(b)-2:], uint16(len(t)))
		case uint32(len(t)) <= math.MaxUint32:
			b = append(b, byte(mpBin32), 0, 0, 0, 0)
			binary.BigEndian.PutUint32(b[len(b)-4:], uint32(len(t)))
		}
		return append(b, t...), nil
	case []int:
		b, err := appendMsgpackArray(b, len(t))
		if err != nil {
			return nil, err
		}
		for _, n := range t {
			b = appendMsgpackInt64(b, int64(n))
		}
		return b, nil
	case []int64:
		b, err := appendMsgpackArray(b, len(t))
		if err != nil {
			return nil, err
		}
		for _, n := range t {
			b = appendMsgpackInt64(b, n)
		}
		return b, nil
	case []string:
		b, err := appendMsgpackArray(b, len(t))
		if err != nil {
			return nil, err
		}
		for _, s := range t {
			b, err = appendMsgpackString(b, s)
			if err != nil {
				return nil, err
			}
		}
		return b, nil
	default:
		return nil, ErrInvalidData
	}
}

// MsgPack implements Formatter for msgpack format.
//
// https://github.com/msgpack/msgpack/blob/master/spec.md
type MsgPack struct {
	// Utsname can normally be left blank.
	// If not empty, the string is used instead of the hostname.
	// Utsname must match this regexp: ^[a-z][a-z0-9-]*$
	Utsname string
}

// String returns "msgpack".
func (m MsgPack) String() string {
	return "msgpack"
}

// Format implements Formatter.Format.
func (m MsgPack) Format(b []byte, l *Logger, t time.Time, severity int, msg string,
	fields map[string]interface{}) ([]byte, error) {
	b = append(b, byte(mpFixArray+3))
	b, err := appendMsgpack(b, l.Topic())
	if err != nil {
		return nil, err
	}
	b, err = appendMsgpack(b, t.Unix())
	if err != nil {
		return nil, err
	}

	// the log record consists of these objects:
	//     logged_at, severity, utsname, message, objects in fields,
	//     and objects in l.defaults excluding conflicting keys.
	var nFields uint64
	nFields += 4
	for k := range fields {
		if !ReservedKey(k) {
			nFields++
		}
	}
	for k := range l.Defaults() {
		if ReservedKey(k) {
			continue
		}
		if _, ok := fields[k]; ok {
			continue
		}
		nFields++
	}
	if nFields > math.MaxUint16 {
		return nil, ErrTooLarge
	}

	if nFields <= 15 {
		b = append(b, byte(mpFixMap+nFields))
	} else {
		b = append(b, byte(mpMap16), 0, 0)
		binary.BigEndian.PutUint16(b[len(b)-2:], uint16(nFields))
	}

	// logged_at
	b, err = appendMsgpack(b, FnLoggedAt)
	if err != nil {
		return nil, err
	}
	b, err = appendMsgpack(b, t.UnixNano()/1000)
	if err != nil {
		return nil, err
	}

	// severity
	b, err = appendMsgpack(b, FnSeverity)
	if err != nil {
		return nil, err
	}
	b, err = appendMsgpack(b, severity)
	if err != nil {
		return nil, err
	}

	// utsname
	b, err = appendMsgpack(b, FnUtsname)
	if err != nil {
		return nil, err
	}
	if len(m.Utsname) > 0 {
		b, err = appendMsgpack(b, m.Utsname)
	} else {
		b, err = appendMsgpack(b, utsname)
	}
	if err != nil {
		return nil, err
	}

	b, err = appendMsgpack(b, FnMessage)
	if err != nil {
		return nil, err
	}
	// message
	if len(b)+len(msg) > maxLogSize {
		return nil, ErrTooLarge
	}
	b, err = appendMsgpack(b, msg)
	if err != nil {
		return nil, err
	}

	// fields
	for k, v := range fields {
		if ReservedKey(k) {
			continue
		}
		if len(b)+len(k) > maxLogSize {
			return nil, ErrTooLarge
		}
		b, err = appendMsgpack(b, k)
		if err != nil {
			return nil, err
		}

		b, err = appendMsgpack(b, v)
		if err != nil {
			return nil, err
		}
	}

	// defaults
	for k, v := range l.Defaults() {
		if ReservedKey(k) {
			continue
		}
		if _, ok := fields[k]; ok {
			continue
		}
		if len(b)+len(k) > maxLogSize {
			return nil, ErrTooLarge
		}
		b, err = appendMsgpack(b, k)
		if err != nil {
			return nil, err
		}

		b, err = appendMsgpack(b, v)
		if err != nil {
			return nil, err
		}
	}

	return b, nil
}
