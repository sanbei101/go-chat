package model

import (
	"encoding/json/jsontext"
	"fmt"

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

var validTypes = map[MessageType]bool{
	MessageTypeText:   true,
	MessageTypeImage:  true,
	MessageTypeVideo:  true,
	MessageTypeFile:   true,
	MessageTypeSystem: true,
}

func (t MessageType) IsValid() bool {
	_, ok := validTypes[t]
	return ok
}

type MessageDTO struct {
	ClientMsgID  uuid.UUID      `json:"client_msg_id"`
	SenderID     uuid.UUID      `json:"sender_id"`
	RoomID       uuid.UUID      `json:"room_id"`
	ReplyToMsgID *uuid.UUID     `json:"reply_to_msg_id"`
	MsgType      MessageType    `json:"msg_type"`
	Payload      jsontext.Value `json:"payload"`
	Ext          jsontext.Value `json:"ext"`
}

func (dto *MessageDTO) Validate() error {
	if dto.ClientMsgID == uuid.Nil {
		return fmt.Errorf("client_msg_id is required")
	}
	if dto.SenderID == uuid.Nil {
		return fmt.Errorf("sender_id is required")
	}
	if dto.RoomID == uuid.Nil {
		return fmt.Errorf("room_id is required")
	}
	if dto.MsgType == "" {
		return fmt.Errorf("msg_type is required")
	}
	if !dto.MsgType.IsValid() {
		return fmt.Errorf("invalid msg_type")
	}

	if len(dto.Payload) == 0 {
		return fmt.Errorf("payload is required")
	}
	return nil
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
