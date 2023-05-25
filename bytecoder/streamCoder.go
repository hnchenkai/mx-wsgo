package bytecoder

import (
	"bytes"
	"compress/gzip"
	_ "embed"

	"google.golang.org/protobuf/proto"
)

type StreamCoder []byte

func (c *StreamCoder) UnmarshalV1() (*Messagev1, error) {
	msg := Messagev1{}
	if err := proto.Unmarshal(*c, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func MarshalV2(reqId int64, cmd int32, body []byte, metadata map[string]string) StreamCoder {
	if bt, err := proto.Marshal(&Messagev2{
		Version:   Version_VERSION_2,
		RequestId: reqId,
		Cmd:       cmd,
		Body:      body,
		Header:    metadata,
	}); err != nil {
		return nil
	} else {
		return bt
	}
}

func MarshalV0(reqId int64, code int32, message []byte, header map[string]string) StreamCoder {
	c, _ := proto.Marshal(&Messagev0{
		Version:   Version_VERSION_0_UNSPECIFIED,
		RequestId: reqId,
		Code:      code,
		Message:   message,
		Header:    header,
	})

	return c
}

func (c *StreamCoder) UnmarshalV2() (*Messagev2, error) {
	msg := Messagev2{}
	if err := proto.Unmarshal(*c, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (c *StreamCoder) Byte() []byte {
	return *c
}

func (c *StreamCoder) Version() Version {
	// TODO: 这里推荐使用pb的格式
	v := Message{}
	if err := proto.Unmarshal(*c, &v); err != nil {
		return 0
	}

	return v.GetVersion()
}

func (c *StreamCoder) Gzip() error {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	defer gzipWriter.Close()

	if _, err := gzipWriter.Write(*c); err != nil {
		return err
	}

	if err := gzipWriter.Close(); err != nil {
		return err
	}
	*c = buf.Bytes()
	return nil
}

func (msg *StreamCoder) UnGzip() error {
	gzipReader, err := gzip.NewReader(bytes.NewReader(*msg))
	if err != nil {
		return err
	}

	defer gzipReader.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(gzipReader); err != nil {
		return err
	}
	*msg = buf.Bytes()
	return nil
}

func (message *StreamCoder) DecodeWS() {
	sum32 := 0
	for _, v := range *message {
		if v == 32 || v == 10 {
			sum32++
		}
	}

	if sum32 == 0 {
		return
	}

	var buff bytes.Buffer
	jump := false
	for _, v := range *message {
		if jump {
			if v == 1 {
				buff.WriteByte(10)
			} else {
				buff.WriteByte(32)
			}
			jump = false
		} else {
			if v == 32 {
				jump = true
			} else {
				buff.WriteByte(v)
			}
		}
	}

	*message = buff.Bytes()
}

func (message *StreamCoder) EncodeWS() {
	sum32 := 0
	for _, v := range *message {
		if v == 32 || v == 10 {
			sum32++
		}
	}

	if sum32 == 0 {
		return
	}

	var buff bytes.Buffer
	for _, v := range *message {
		switch v {
		case 10:
			buff.WriteByte(32)
			buff.WriteByte(1)
		case 32:
			buff.WriteByte(32)
			buff.WriteByte(2)
		default:
			buff.WriteByte(v)
		}
	}

	*message = buff.Bytes()
}

//go:embed message.proto
var pbfile StreamCoder

func ShowProtoFile() StreamCoder {
	return pbfile
}
