package config

import (
	"github.com/Mersad-Moghaddam/shared/messagequeue"
)

var RabbitMQ *messagequeue.RabbitMQ

// InitRabbitMQ initializes the RabbitMQ connection
func InitRabbitMQ() *messagequeue.RabbitMQ {
	var err error
	RabbitMQ, err = messagequeue.NewRabbitMQ()
	if err != nil {
		panic(err)
	}
	return RabbitMQ
}
