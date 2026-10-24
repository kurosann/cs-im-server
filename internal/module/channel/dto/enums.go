package dto

type SenderRole string

const (
	SenderRoleCustomer SenderRole = "customer"
	SenderRoleAgent    SenderRole = "agent"
)

func (role SenderRole) Valid() bool {
	return role == SenderRoleCustomer || role == SenderRoleAgent
}

type MessageType string

const (
	MessageTypeText  MessageType = "text"
	MessageTypeImage MessageType = "image"
	MessageTypeFile  MessageType = "file"
	MessageTypeVideo MessageType = "video"
)

func (messageType MessageType) Valid() bool {
	return messageType == MessageTypeText || messageType == MessageTypeImage ||
		messageType == MessageTypeFile || messageType == MessageTypeVideo
}

type MessageStatus string

const (
	MessageStatusReceived  MessageStatus = "received"
	MessageStatusSending   MessageStatus = "sending"
	MessageStatusSent      MessageStatus = "sent"
	MessageStatusDelivered MessageStatus = "delivered"
	MessageStatusRead      MessageStatus = "read"
	MessageStatusFailed    MessageStatus = "failed"
)

func (status MessageStatus) Valid() bool {
	return status == MessageStatusReceived || status == MessageStatusSending ||
		status == MessageStatusSent ||
		status == MessageStatusDelivered || status == MessageStatusRead ||
		status == MessageStatusFailed
}

type HistoryRequestStatus string

const (
	HistoryRequestStatusCompleted HistoryRequestStatus = "completed"
	HistoryRequestStatusFailed    HistoryRequestStatus = "failed"
)

func (status HistoryRequestStatus) Valid() bool {
	return status == HistoryRequestStatusCompleted || status == HistoryRequestStatusFailed
}
