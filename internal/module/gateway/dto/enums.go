package dto

type MessageSenderRole string

const (
	MessageSenderRoleCustomer MessageSenderRole = "customer"
	MessageSenderRoleAgent    MessageSenderRole = "agent"
	MessageSenderRoleAI       MessageSenderRole = "ai"
)

func (role MessageSenderRole) Valid() bool {
	return role == MessageSenderRoleCustomer || role == MessageSenderRoleAgent ||
		role == MessageSenderRoleAI
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
	HistoryRequestStatusPending   HistoryRequestStatus = "pending"
	HistoryRequestStatusCompleted HistoryRequestStatus = "completed"
	HistoryRequestStatusFailed    HistoryRequestStatus = "failed"
)

func (status HistoryRequestStatus) Valid() bool {
	return status == HistoryRequestStatusPending || status == HistoryRequestStatusCompleted ||
		status == HistoryRequestStatusFailed
}
