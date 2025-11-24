package utils

import (
	"github.com/golang/mock/gomock"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

// Protobuf returns a Matcher that relies upon proto.Equal to compare Protobuf messages
// Example usage with mocked request:
//
// example.EXPECT().ExampleRequest(Protobuf(requestMsg)).Return(responseMsg, nil).AnyTimes()
func Protobuf(msg proto.Message) gomock.Matcher {
	return &ProtobufMatcher{msg}
}

type ProtobufMatcher struct {
	msg proto.Message
}

var _ gomock.Matcher = &ProtobufMatcher{}

func (p *ProtobufMatcher) Matches(x interface{}) bool {
	otherMsg, ok := x.(proto.Message)
	if !ok {
		return false
	}
	return proto.Equal(p.msg, otherMsg)
}

func (p *ProtobufMatcher) String() string {
	return prototext.Format(p.msg)
}
