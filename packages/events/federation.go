package events

// FederationOutboundEvent is published when a message must be delivered to
// a remote Relay server.
// Published by: Messaging Service (when channel has remote participants).
// Consumed by: Federation Service.
type FederationOutboundEvent struct {
	TargetServerID int64  `json:"target_server_id"` // FK into federation_servers
	TargetDomain   string `json:"target_domain"`    // e.g. "other.example.com"
	ChannelID      int64  `json:"channel_id"`
	// Envelope is the signed S2S payload; Federation Service delivers it as-is.
	Envelope []byte `json:"envelope"`
}
