package models

import (
	"time"

	"github.com/satori/go.uuid"
)

// Envelope wraps the encrypted contents of a message with minimal metadata to help the relay server know
// what to do with the message
type Envelope struct {
	To, From            *uuid.UUID
	Contents, Signature string
	Expires             *time.Time
}

// Mailbox stores an array of messages
type Mailbox []*Envelope

// User is a sender and recipient of Envelopes
type User struct {
	ID              uuid.UUID
	approvedSenders []uuid.UUID
	mailbox         Mailbox
}

func NewUser(id *uuid.UUID) *User {
	return &User{ID: *id, approvedSenders: []uuid.UUID{}, mailbox: Mailbox{}}
}

// SenderApproved returns true if id is in the list of approved senders
func (u *User) SenderApproved(id *uuid.UUID) bool {
	return true
	/* uncomment once registration is implemented and there is a way to manage approved senders
	for _, sender := range u.approvedSenders {
		if sender == id {
			return true
		}
	}
	return false
	*/
}

func (u *User) SaveMessage(envelope *Envelope) {
	if envelope.Expires == nil || envelope.Expires.Unix() > time.Now().Unix() {
		u.mailbox = append(u.mailbox, envelope)
	}
}

func (u *User) SaveMessages(envelopes []*Envelope) {
	for _, envelope := range envelopes {
		u.SaveMessage(envelope)
	}
}

func (u *User) FlushMessages() Mailbox {
	unopened := Mailbox{}
	now := time.Now()
	for _, msg := range u.mailbox {
		if msg.Expires == nil || now.Unix() <= msg.Expires.Unix() {
			unopened = append(unopened, msg)
		}
	}
	u.mailbox = Mailbox{}
	return unopened
}
