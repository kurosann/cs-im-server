package service

import "time"

type App struct {
	ID              string    `json:"appId"`
	ServiceKey      string    `json:"serviceKey"`
	DisplayName     string    `json:"displayName"`
	Kind            string    `json:"kind"`
	Version         string    `json:"version,omitempty"`
	Capabilities    []string  `json:"capabilities,omitempty"`
	Status          string    `json:"status"`
	AccessToken     string    `json:"accessToken"`
	LeaseTTLSeconds int       `json:"leaseTtlSeconds"`
	LastHeartbeatAt time.Time `json:"lastHeartbeatAt"`
	LeaseExpiresAt  time.Time `json:"leaseExpiresAt"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type CustomerIdentity struct {
	Type        string    `json:"type"`
	ExternalID  string    `json:"externalId"`
	DisplayName string    `json:"displayName,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Customer struct {
	ID         string             `json:"customerId"`
	Name       string             `json:"name"`
	Channel    string             `json:"channel,omitempty"`
	Status     string             `json:"status"`
	Identities []CustomerIdentity `json:"identities,omitempty"`
	CreatedAt  time.Time          `json:"createdAt"`
	UpdatedAt  time.Time          `json:"updatedAt"`
}

type Conversation struct {
	ID          string     `json:"conversationId"`
	AppID       string     `json:"appId"`
	CustomerID  string     `json:"customerId"`
	Channel     string     `json:"channel,omitempty"`
	Subject     string     `json:"subject,omitempty"`
	Status      string     `json:"status"`
	Assignee    string     `json:"assignee,omitempty"`
	ForwardedTo string     `json:"forwardedTo,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	ClosedAt    *time.Time `json:"closedAt,omitempty"`
}

type MessageSender struct {
	Role string `json:"role"`
	ID   string `json:"id"`
}

type MessageSource struct {
	AppID      string `json:"appId,omitempty"`
	ExternalID string `json:"externalId,omitempty"`
}

type Message struct {
	ID             string        `json:"messageId"`
	ConversationID string        `json:"conversationId"`
	CustomerID     string        `json:"customerId"`
	Sender         MessageSender `json:"sender"`
	Source         MessageSource `json:"source,omitempty"`
	Type           string        `json:"type"`
	Text           string        `json:"text,omitempty"`
	Status         string        `json:"status"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`
}

type Channel struct {
	ID           string         `json:"channelId"`
	Name         string         `json:"name"`
	Type         string         `json:"type"`
	Endpoint     string         `json:"endpoint,omitempty"`
	Config       map[string]any `json:"config,omitempty"`
	Status       string         `json:"status"`
	LastTestedAt time.Time      `json:"lastTestedAt"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
}

type Webhook struct {
	ID        string    `json:"webhookId"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Secret    string    `json:"secret"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type AIConfig struct {
	Prompt              string `json:"prompt"`
	HistoryMessageCount int    `json:"historyMessageCount"`
	AutoReplyEnabled    bool   `json:"autoReplyEnabled"`
	ReportingEnabled    bool   `json:"reportingEnabled"`
}

type AIExecution struct {
	ID             string    `json:"executionId"`
	ConversationID string    `json:"conversationId"`
	MessageID      string    `json:"messageId"`
	Action         string    `json:"action"`
	Result         string    `json:"result,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

type Delivery struct {
	ID             string    `json:"deliveryId"`
	ConversationID string    `json:"conversationId"`
	MessageID      string    `json:"messageId"`
	ChannelID      string    `json:"channelId"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
