package model

import (
	"encoding/json/jsontext"

	"github.com/google/uuid"
)

type MessageType string

const (
	MessageTypeText   MessageType = "text"
	MessageTypeImage  MessageType = "image"
	MessageTypeVideo  MessageType = "video"
	MessageTypeFile   MessageType = "file"
	MessageTypeSystem MessageType = "system"
)

type MessageDTO struct {
	ClientMsgID  uuid.UUID      `json:"client_msg_id"`
	SenderID     uuid.UUID      `json:"sender_id"`
	RoomID       uuid.UUID      `json:"room_id"`
	ReplyToMsgID *uuid.UUID     `json:"reply_to_msg_id"`
	MsgType      MessageType    `json:"msg_type"`
	Payload      jsontext.Value `json:"payload"`
	Ext          jsontext.Value `json:"ext"`
}

type SendMsgAckVO struct {
	ClientMsgID uuid.UUID `json:"client_msg_id"`
	MsgID       uuid.UUID `json:"msg_id"`
	ServerTime  int64     `json:"server_time"`
}

type MessageVO struct {
	MsgID        uuid.UUID      `json:"msg_id"`
	SenderID     uuid.UUID      `json:"sender_id"`
	RoomID       uuid.UUID      `json:"room_id"`
	ReplyToMsgID *uuid.UUID     `json:"reply_to_msg_id,omitempty"`
	MsgType      MessageType    `json:"msg_type"`
	ServerTime   int64          `json:"server_time"`
	Payload      jsontext.Value `json:"payload"`
	Ext          jsontext.Value `json:"ext,omitempty"`
}
