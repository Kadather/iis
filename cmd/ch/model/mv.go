package mv

import (
	"fmt"
	"html/template"

	"github.com/coyove/iis/cmd/ch/ident"
	"github.com/gogo/protobuf/proto"
)

type Article struct {
	ID          []byte `protobuf:"bytes,1,opt"`
	Timeline    []byte `protobuf:"bytes,12,opt"`
	Index       int64  `protobuf:"varint,2,opt"`
	Replies     int64  `protobuf:"varint,4,opt"`
	Locked      bool   `protobuf:"varint,5,opt"`
	Highlighted bool   `protobuf:"varint,6,opt"`
	Saged       bool   `protobuf:"varint,100,opt"`
	Announce    bool   `protobuf:"varint,7,opt"`
	Title       string `protobuf:"bytes,8,opt"`
	Content     string `protobuf:"bytes,9,opt"`
	Author      string `protobuf:"bytes,10,opt"`
	IP          string `protobuf:"bytes,11,opt"`
	Category    string `protobuf:"bytes,13,opt"`
	Views       int64  `protobuf:"varint,14,opt"`
	CreateTime  uint32 `protobuf:"fixed32,15,opt"`
	ReplyTime   uint32 `protobuf:"fixed32,16,opt"`
}

func (a *Article) Reset() { *a = Article{} }

func (a *Article) String() string { return proto.CompactTextString(a) }

func (a *Article) ProtoMessage() {}

func (a *Article) Parent() []byte {
	i, _ := ident.ParseID(a.ID).RIndexParent()
	return i.Marshal()
}

func (a *Article) ContentHTML() template.HTML {
	return template.HTML(sanText(a.Content))
}

func (a *Article) MarshalA() []byte {
	b, _ := proto.Marshal(a)
	return b
}

func (a *Article) UnmarshalA(b []byte) error {
	err := proto.Unmarshal(b, a)
	if a.ID == nil {
		return fmt.Errorf("failed to unmarshal")
	}
	return err
}