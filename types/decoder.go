package types

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"strings"
	"time"

	"github.com/redcuckoo/eosgo/ecc"
)

var TypeSize = struct {
	Byte           int
	Int8           int
	UInt16         int
	Int16          int
	UInt32         int
	UInt64         int
	SHA256Bytes    int
	PublicKey      int
	Signature      int
	Tstamp         int
	BlockTimestamp int
	CurrencyName   int
	Bool           int
}{
	Byte:           1,
	Int8:           1,
	UInt16:         2,
	Int16:          2,
	UInt32:         4,
	UInt64:         8,
	SHA256Bytes:    32,
	PublicKey:      34,
	Signature:      66,
	Tstamp:         8,
	BlockTimestamp: 4,
	CurrencyName:   7,
	Bool:           1,
}

var RegisteredActions = map[AccountName]map[ActionName]reflect.Type{}

// Registers Action objects..
func RegisterAction(accountName AccountName, actionName ActionName, obj interface{}) {
	// TODO: lock or som'th.. unless we never call after boot time..
	if RegisteredActions[accountName] == nil {
		RegisteredActions[accountName] = make(map[ActionName]reflect.Type)
	}
	RegisteredActions[accountName][actionName] = reflect.TypeOf(obj)
}

// Decoder implements the EOS unpacking, similar to FC_BUFFER
type Decoder struct {
	data               []byte
	pos                int
	decodeP2PMessage   bool
	decodeTransactions bool
	decodeActions      bool
}

//var prefix = make([]string, 0)

var Debug bool

var print = func(s string) {
	if Debug {
		//for _, s := range prefix {
		//fmt.Print(s)
		//}
		fmt.Print(s)
	}
}
var println = func(args ...interface{}) {
	print(fmt.Sprintf("%s\n", args...))
}

func NewDecoder(data []byte) *Decoder {
	return &Decoder{
		data:               data,
		decodeP2PMessage:   true,
		decodeTransactions: true,
		decodeActions:      true,
	}
}

func (d *Decoder) DecodeP2PMessage(decode bool) {
	d.decodeP2PMessage = decode
}

func (d *Decoder) DecodeActions(decode bool) {
	d.decodeActions = decode
}

func (d *Decoder) Decode(v interface{}) (err error) {
	rv := reflect.Indirect(reflect.ValueOf(v))
	if !rv.CanAddr() {
		return errors.New("decode, can only Decode to pointer type")
	}
	t := rv.Type()

	println(fmt.Sprintf("Decode type [%T]", v))
	if !rv.CanAddr() {
		return errors.New("binary: can only Decode to pointer type")
	}

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		newRV := reflect.New(t)
		rv.Set(newRV)
		rv = reflect.Indirect(newRV)
	}

	switch realV := v.(type) {
	case *string:
		s, e := d.ReadString()
		if e != nil {
			err = e
			return
		}
		rv.SetString(s)
		return
	case *Name, *AccountName, *PermissionName, *ActionName, *TableName, *ScopeName:
		var n uint64
		n, err = d.ReadUint64()
		name := NameToString(n)
		println(fmt.Sprintf("readName [%s]", name))
		rv.SetString(name)
		return
	case *byte, *P2PMessageType, *TransactionStatus, *CompressionType, *IDListMode, *GoAwayReason:
		var n byte
		n, err = d.ReadByte()
		rv.SetUint(uint64(n))
		return
	case *int16:
		var n int16
		n, err = d.ReadInt16()
		rv.SetInt(int64(n))
		return
	case *uint16:
		var n uint16
		n, err = d.ReadUint16()
		rv.SetUint(uint64(n))
		return
	case *uint32:
		var n uint32
		n, err = d.ReadUint32()
		rv.SetUint(uint64(n))
		return
	case *uint64:
		var n uint64
		n, err = d.ReadUint64()
		rv.SetUint(n)
		return
	case *Varuint32:
		var r uint64
		r, err = d.ReadUvarint()
		rv.SetUint(r)
		return
	case *bool:
		var r bool
		r, err = d.ReadBool()
		rv.SetBool(r)
		return
	case *Bool:
		var r bool
		r, err = d.ReadBool()
		rv.SetBool(r)
		return
	case *HexBytes:
		var data []byte
		data, err = d.ReadByteArray()
		rv.SetBytes(data)
		return
	case *[]byte:
		var data []byte
		data, err = d.ReadByteArray()
		rv.SetBytes(data)
		return
	case *SHA256Bytes:
		var s SHA256Bytes
		s, err = d.ReadSHA256Bytes()
		rv.SetBytes(s)
		return
	case *ecc.PublicKey:
		var p ecc.PublicKey
		p, err = d.ReadPublicKey()
		rv.Set(reflect.ValueOf(p))
		return
	case *ecc.Signature:
		var s ecc.Signature
		s, err = d.ReadSignature()
		rv.Set(reflect.ValueOf(s))
		return
	case *Tstamp:
		var ts Tstamp
		ts, err = d.ReadTstamp()
		rv.Set(reflect.ValueOf(ts))
		return
	case *BlockTimestamp:
		var bt BlockTimestamp
		bt, err = d.ReadBlockTimestamp()
		rv.Set(reflect.ValueOf(bt))
		return
	case *JSONTime:
		var jt JSONTime
		jt, err = d.readJSONTime()
		rv.Set(reflect.ValueOf(jt))
		return
	case *CurrencyName:
		var cur CurrencyName
		cur, err = d.readCurrencyName()
		rv.Set(reflect.ValueOf(cur))
		return
	case *Asset:
		var asset Asset
		asset, err = d.readAsset()
		rv.Set(reflect.ValueOf(asset))
		return

	case *TransactionWithID:

		t, e := d.ReadByte()
		if err != nil {
			err = fmt.Errorf("decode: TransactionWithID failed to read type byte: %s", e)
			return
		}

		println(fmt.Sprintf("Type byte value : %d", t))

		if t == 0 {
			id, e := d.ReadSHA256Bytes()
			if err != nil {
				err = fmt.Errorf("decode: TransactionWithID failed to read id: %s", e)
				return
			}

			trx := TransactionWithID{ID: id}
			rv.Set(reflect.ValueOf(trx))
			return nil

		} else {
			packedTrx := &PackedTransaction{}
			d.Decode(packedTrx)
			trx := TransactionWithID{Packed: packedTrx}
			rv.Set(reflect.ValueOf(trx))
			return nil
		}

	case **OptionalProducerSchedule:
		isPresent, e := d.ReadByte()
		if e != nil {
			err = fmt.Errorf("decode: OptionalProducerSchedule isPresent, %s", e)
			return
		}

		if isPresent == 0 {
			println("Skipping optional OptionalProducerSchedule")
			*realV = nil
			return
		}

	case **Action:
		err = d.decodeStruct(v, t, rv)
		if err != nil {
			return
		}
		action := rv.Interface().(Action)

		if d.decodeActions {
			err = d.readActionData(&action)
		}

		rv.Set(reflect.ValueOf(action))
		return

	case *P2PMessageEnvelope:

		envelope, e := d.readP2PMessageEnvelope()
		if e != nil {
			err = fmt.Errorf("decode, %s", e)
			return
		}

		if d.decodeP2PMessage {
			attr, ok := envelope.Type.reflectTypes()
			if !ok {
				return fmt.Errorf("decode, unknown p2p message type [%d]", envelope.Type)
			}
			msg := reflect.New(attr.ReflectType)
			subDecoder := NewDecoder(envelope.Payload)

			err = subDecoder.Decode(msg.Interface())

			decoded := msg.Interface().(P2PMessage)
			envelope.P2PMessage = decoded
		}

		rv.Set(reflect.ValueOf(*envelope))

		return
	}

	switch t.Kind() {
	case reflect.Array:
		print("Array")
		len := t.Len()
		for i := 0; i < int(len); i++ {
			if err = d.Decode(rv.Index(i).Addr().Interface()); err != nil {
				return
			}
		}
		return

	case reflect.Slice:
		print("Reading Slice length ")
		var l uint64
		if l, err = d.ReadUvarint(); err != nil {
			return
		}
		println(fmt.Sprintf("Slice [%T] of length: %d", v, l))
		rv.Set(reflect.MakeSlice(t, int(l), int(l)))
		for i := 0; i < int(l); i++ {
			if err = d.Decode(rv.Index(i).Addr().Interface()); err != nil {
				return
			}
		}

	case reflect.Struct:

		err = d.decodeStruct(v, t, rv)
		if err != nil {
			return
		}

	case reflect.Map:
		//fmt.Println("Map")
		var l uint64
		if l, err = d.ReadUvarint(); err != nil {
			return
		}
		kt := t.Key()
		vt := t.Elem()
		rv.Set(reflect.MakeMap(t))
		for i := 0; i < int(l); i++ {
			kv := reflect.Indirect(reflect.New(kt))
			if err = d.Decode(kv.Addr().Interface()); err != nil {
				return
			}
			vv := reflect.Indirect(reflect.New(vt))
			if err = d.Decode(vv.Addr().Interface()); err != nil {
				return
			}
			rv.SetMapIndex(kv, vv)
		}

	default:
		return errors.New("decode, unsupported type " + t.String())
	}

	return
}

func (d *Decoder) decodeStruct(v interface{}, t reflect.Type, rv reflect.Value) (err error) {
	l := rv.NumField()

	//prefix = append(prefix, "     ")
	for i := 0; i < l; i++ {

		if tag := t.Field(i).Tag.Get("eos"); tag == "-" {
			continue
		}

		if v := rv.Field(i); v.CanSet() && t.Field(i).Name != "_" {
			iface := v.Addr().Interface()
			println(fmt.Sprintf("Field name: %s", t.Field(i).Name))
			if err = d.Decode(iface); err != nil {
				return
			}
		}
	}
	//prefix = prefix[:len(prefix)-1]
	return
}

var ErrVarIntBufferSize = errors.New("varint: invalid buffer size")

func (d *Decoder) ReadUvarint() (uint64, error) {

	l, read := binary.Uvarint(d.data[d.pos:])
	if read <= 0 {
		println(fmt.Sprintf("ReadUvarint [%d]", l))
		return l, ErrVarIntBufferSize
	}

	d.pos += read
	println(fmt.Sprintf("ReadUvarint [%d]", l))
	return l, nil
}

func (d *Decoder) ReadByteArray() (out []byte, err error) {

	l, err := d.ReadUvarint()
	if err != nil {
		return nil, err
	}

	if len(d.data) < d.pos+int(l) {
		return nil, fmt.Errorf("byte array: varlen=%d, missing %d bytes", l, d.pos+int(l)-len(d.data))
	}

	out = d.data[d.pos : d.pos+int(l)]
	d.pos += int(l)

	println(fmt.Sprintf("ReadByteArray [%s]", hex.EncodeToString(out)))
	return
}

func (d *Decoder) ReadByte() (out byte, err error) {

	if d.Remaining() < TypeSize.Byte {
		err = fmt.Errorf("byte required [1] byte, remaining [%d]", d.Remaining())
		return
	}

	out = d.data[d.pos]
	d.pos++
	println(fmt.Sprintf("ReadByte [%d]", out))
	return
}

func (d *Decoder) ReadBool() (out bool, err error) {

	if d.Remaining() < TypeSize.Bool {
		err = fmt.Errorf("bool required [%d] byte, remaining [%d]", TypeSize.Bool, d.Remaining())
		return
	}

	b, err := d.ReadByte()

	if err != nil {
		err = fmt.Errorf("ReadBool, %s", err)
	}
	out = b != 0
	return

}

func (d *Decoder) ReadUint16() (out uint16, err error) {
	if d.Remaining() < TypeSize.UInt16 {
		err = fmt.Errorf("uint16 required [%d] bytes, remaining [%d]", TypeSize.UInt16, d.Remaining())
		return
	}

	out = binary.LittleEndian.Uint16(d.data[d.pos:])
	d.pos += TypeSize.UInt16
	return
}

func (d *Decoder) ReadInt16() (out int16, err error) {
	n, err := d.ReadUint16()
	out = int16(n)
	return
}
func (d *Decoder) readInt64() (out int64, err error) {
	n, err := d.ReadUint64()
	out = int64(n)
	return
}

func (d *Decoder) ReadUint32() (out uint32, err error) {
	if d.Remaining() < TypeSize.UInt32 {
		err = fmt.Errorf("uint32 required [%d] bytes, remaining [%d]", TypeSize.UInt32, d.Remaining())
		return
	}

	out = binary.LittleEndian.Uint32(d.data[d.pos:])
	d.pos += TypeSize.UInt32
	println(fmt.Sprintf("ReadUint32 [%d]", out))
	return
}

func (d *Decoder) ReadUint64() (out uint64, err error) {
	if d.Remaining() < TypeSize.UInt64 {
		err = fmt.Errorf("uint64 required [%d] bytes, remaining [%d]", TypeSize.UInt64, d.Remaining())
		return
	}

	data := d.data[d.pos : d.pos+TypeSize.UInt64]
	out = binary.LittleEndian.Uint64(data)
	d.pos += TypeSize.UInt64
	println(fmt.Sprintf("ReadUint64 [%d] [%s]", out, hex.EncodeToString(data)))
	return
}

func (d *Decoder) ReadString() (out string, err error) {
	data, err := d.ReadByteArray()
	out = string(data)
	println(fmt.Sprintf("ReadString [%s]", out))
	return
}

func (d *Decoder) ReadSHA256Bytes() (out SHA256Bytes, err error) {

	if d.Remaining() < TypeSize.SHA256Bytes {
		err = fmt.Errorf("sha256 required [%d] bytes, remaining [%d]", TypeSize.SHA256Bytes, d.Remaining())
		return
	}

	out = SHA256Bytes(d.data[d.pos : d.pos+TypeSize.SHA256Bytes])
	d.pos += TypeSize.SHA256Bytes
	println(fmt.Sprintf("ReadSHA256Bytes [%s]", hex.EncodeToString(out)))
	return
}

func (d *Decoder) ReadPublicKey() (out ecc.PublicKey, err error) {

	if d.Remaining() < TypeSize.PublicKey {
		err = fmt.Errorf("publicKey required [%d] bytes, remaining [%d]", TypeSize.PublicKey, d.Remaining())
		return
	}
	out = ecc.PublicKey{
		Curve:   ecc.CurveID(d.data[d.pos]),                 // 1 byte
		Content: d.data[d.pos+1 : d.pos+TypeSize.PublicKey], // 33 bytes
	}
	d.pos += TypeSize.PublicKey
	println(fmt.Sprintf("ReadPublicKey [curve=%d, content=%s]", out.Curve, hex.EncodeToString(out.Content)))
	return
}

func (d *Decoder) ReadSignature() (out ecc.Signature, err error) {
	if d.Remaining() < TypeSize.Signature {
		err = fmt.Errorf("signature required [%d] bytes, remaining [%d]", TypeSize.Signature, d.Remaining())
		return
	}
	out = ecc.Signature{
		Curve:   ecc.CurveID(d.data[d.pos]),                 // 1 byte
		Content: d.data[d.pos+1 : d.pos+TypeSize.Signature], // 65 bytes
	}
	d.pos += TypeSize.Signature
	println(fmt.Sprintf("ReadSignature [curve=%d, content=%s]", out.Curve, hex.EncodeToString(out.Content)))
	return
}

func (d *Decoder) ReadTstamp() (out Tstamp, err error) {

	if d.Remaining() < TypeSize.Tstamp {
		err = fmt.Errorf("tstamp required [%d] bytes, remaining [%d]", TypeSize.Tstamp, d.Remaining())
		return
	}

	unixNano, err := d.ReadUint64()
	out.Time = time.Unix(0, int64(unixNano))
	println(fmt.Sprintf("ReadTstamp [%s]", out))
	return
}

func (d *Decoder) ReadBlockTimestamp() (out BlockTimestamp, err error) {
	if d.Remaining() < TypeSize.BlockTimestamp {
		err = fmt.Errorf("blockTimestamp required [%d] bytes, remaining [%d]", TypeSize.BlockTimestamp, d.Remaining())
		return
	}
	n, err := d.ReadUint32()
	out.Time = time.Unix(int64(n)+946684800, 0)
	return
}

func (d *Decoder) readJSONTime() (jsonTime JSONTime, err error) {
	n, err := d.ReadUint32()
	jsonTime = JSONTime{time.Unix(int64(n), 0).UTC()}
	return
}

func (d *Decoder) readCurrencyName() (out CurrencyName, err error) {

	data := d.data[d.pos : d.pos+TypeSize.CurrencyName]
	d.pos += TypeSize.CurrencyName

	out = CurrencyName(strings.TrimRight(string(data), "\x00"))
	return
}

func (d *Decoder) readAsset() (out Asset, err error) {

	amount, err := d.readInt64()
	precision, err := d.ReadByte()
	if err != nil {
		return out, fmt.Errorf("readSymbol precision, %s", err)
	}

	data := d.data[d.pos : d.pos+7]
	d.pos += 7

	out = Asset{}
	out.Amount = amount
	out.Precision = precision
	out.Symbol.Symbol = strings.TrimRight(string(data), "\x00")
	return
}

func (d *Decoder) readActionData(action *Action) (err error) {

	actionMap := RegisteredActions[action.Account]

	var decodeInto reflect.Type
	if actionMap != nil {
		objType := actionMap[action.Name]
		println("object type :", objType)
		if objType != nil {
			decodeInto = objType
		}
	}
	if decodeInto == nil {
		return
	}

	println("Reflect type :", decodeInto)
	obj := reflect.New(decodeInto)
	println("obj :", obj)
	err = UnmarshalBinary(action.ActionData.HexData, obj.Interface())
	if err != nil {
		return fmt.Errorf("decoding Action [%s], %s", obj.Type().Name(), err)
	}

	println("Object type :", obj.Interface())
	action.ActionData.Data = obj.Interface()

	return
}

func (d *Decoder) readP2PMessageEnvelope() (out *P2PMessageEnvelope, err error) {

	out = &P2PMessageEnvelope{}
	l, err := d.ReadUint32()
	if err != nil {
		err = fmt.Errorf("p2p envelope length: %s", err)
		return
	}
	out.Length = l
	b, err := d.ReadByte()
	if err != nil {
		err = fmt.Errorf("p2p envelope type: %s", err)
		return
	}
	out.Type = P2PMessageType(b)

	payloadLength := int(l - 1)
	if d.Remaining() < payloadLength {
		err = fmt.Errorf("p2p envelope payload required [%d] bytes, remaining [%d]", l, d.Remaining())
		return
	}
	payload := d.data[d.pos : d.pos+int(payloadLength)]
	d.pos += int(out.Length)

	out.Payload = payload
	return
}

func (d *Decoder) Remaining() int {
	return len(d.data) - d.pos
}

func UnmarshalBinaryReader(reader io.Reader, v interface{}) (err error) {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return
	}
	return UnmarshalBinary(data, v)
}

func UnmarshalBinary(data []byte, v interface{}) (err error) {
	decoder := NewDecoder(data)
	return decoder.Decode(v)
}
