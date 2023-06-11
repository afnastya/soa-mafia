package messenger

import (
	"context"
	"errors"
	"fmt"
	"soa_mafia/pkg/mafia_grpc"
	"time"
)

const (
	RabbitmqUrlPrefix = "amqp://guest:guest@"
	RabbitmqHost      = "localhost"
	RabbitmqPort      = "5672"
)

type MafiaMessenger struct {
	messenger *Messenger

	grpc   *mafia_grpc.MafiaClient
	player *mafia_grpc.PlayerInfo
}

func NewMafiaMessenger(grpc *mafia_grpc.MafiaClient, player *mafia_grpc.PlayerInfo) *MafiaMessenger {
	rabbitmqUrl := RabbitmqUrlPrefix + RabbitmqHost + ":" + RabbitmqPort + "/%2f"

	return &MafiaMessenger{
		NewMessenger(rabbitmqUrl, player.GetSession()),
		grpc,
		player,
	}
}

func (m *MafiaMessenger) Send(msg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	response, err := (*m.grpc).CanChat(ctx, m.player)
	if err != nil {
		fmt.Println("Failed to get responce from grpc server. ", err)
	}

	if !response.GetCanChat() {
		return errors.New("Can't chat right now")
	}

	m.messenger.Send(msg)
	return nil
}

func (m *MafiaMessenger) Receive() chan string {
	return m.messenger.Receive()
}
