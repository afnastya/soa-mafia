package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"

	"soa_mafia/pkg/mafia_grpc"
	mafia_impl "soa_mafia/server/mafia_impl"

	"google.golang.org/grpc"
)

type server struct {
	mafia_grpc.UnimplementedMafiaServer

	session2game map[string]*mafia_impl.Game
}

func (s *server) Join(ctx context.Context, request *mafia_grpc.JoinRequest) (*mafia_grpc.Response, error) {
	log.Println(fmt.Sprintf("Join: %s", request))
	player := request.GetPlayer()
	session := player.GetSession()
	name := player.GetName()

	game, exists := s.session2game[session]
	if !exists {
		game = mafia_impl.NewGame(session)
		s.session2game[session] = game
	}

	err := game.AddPlayer(name)
	return &mafia_grpc.Response{Ok: err == nil}, err
}

func (s *server) Vote(ctx context.Context, request *mafia_grpc.SetVictimRequest) (*mafia_grpc.Response, error) {
	log.Println(fmt.Sprintf("Vote: %s", request))
	player := request.GetPlayer()
	victim := request.GetVictim()
	session := player.GetSession()
	name := player.GetName()

	game, exists := s.session2game[session]
	if !exists {
		return nil, errors.New("No game session: " + session)
	}

	err := game.AddVote(name, victim)
	return &mafia_grpc.Response{Ok: err == nil}, err
}

func (s *server) Kill(ctx context.Context, request *mafia_grpc.SetVictimRequest) (*mafia_grpc.Response, error) {
	log.Println(fmt.Sprintf("Kill: %s", request))
	player := request.GetPlayer()
	victim := request.GetVictim()
	session := player.GetSession()
	name := player.GetName()

	game, exists := s.session2game[session]
	if !exists {
		return nil, errors.New("No game session: " + session)
	}

	err := game.KillPlayer(name, victim)
	return &mafia_grpc.Response{Ok: err == nil}, err
}

func (s *server) CheckIfMafia(ctx context.Context, request *mafia_grpc.SetVictimRequest) (*mafia_grpc.CheckMafiaResponse, error) {
	log.Println(fmt.Sprintf("CheckIfMafia: %s", request))
	player := request.GetPlayer()
	victim := request.GetVictim()
	session := player.GetSession()
	name := player.GetName()

	game, exists := s.session2game[session]
	if !exists {
		return nil, errors.New("No game session: " + session)
	}

	isMafia, err := game.CheckIfMafia(name, victim)
	return &mafia_grpc.CheckMafiaResponse{IsMafia: isMafia}, err
}

func (s *server) GetState(ctx context.Context, player *mafia_grpc.PlayerInfo) (*mafia_grpc.GameState, error) {
	log.Println(fmt.Sprintf("GetState: %s", player))
	session := player.GetSession()

	game, exists := s.session2game[session]
	if !exists {
		return nil, errors.New("No game session: " + session)
	}

	state := game.GetGameState()
	return state, nil
}

func (s *server) CanChat(ctx context.Context, player *mafia_grpc.PlayerInfo) (*mafia_grpc.ChatResponse, error) {
	log.Println(fmt.Sprintf("CanChat: %s", player))
	session := player.GetSession()
	name := player.GetName()

	game, exists := s.session2game[session]
	if !exists {
		return nil, errors.New("No game session: " + session)
	}

	canChat, err := game.CanChat(name)
	return &mafia_grpc.ChatResponse{CanChat: canChat}, err
}

func (s *server) Quit(ctx context.Context, player *mafia_grpc.PlayerInfo) (*mafia_grpc.Response, error) {
	log.Println(fmt.Sprintf("Quit: %s", player))
	session := player.GetSession()
	name := player.GetName()

	game, exists := s.session2game[session]
	if !exists {
		return nil, errors.New("No game session: " + session)
	}

	err := game.DeletePlayer(name)
	return &mafia_grpc.Response{Ok: err == nil}, nil
}

func (s *server) GetNotifications(player *mafia_grpc.PlayerInfo, stream mafia_grpc.Mafia_GetNotificationsServer) error {
	log.Println(fmt.Sprintf("GetNotifications: %s", player))
	session := player.GetSession()
	name := player.GetName()

	game, exists := s.session2game[session]
	if !exists {
		return errors.New("No game session: " + session)
	}

	notifications, err := game.GetNotifications(name)
	if err != nil {
		return err
	}

	for notification := range *notifications {
		log.Printf("Sending to %s notification %s", name, notification.Type)
		err := stream.Send(notification)
		if err != nil {
			log.Printf("Player %s: Notifications connection lost\n", name)
			game.DeletePlayer(name)
		}
	}

	return nil
}

func main() {
	log.Println("Server running ...")
	lis, err := net.Listen("tcp", ":9000")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	mafia_grpc.RegisterMafiaServer(srv, &server{session2game: make(map[string]*mafia_impl.Game)})
	log.Fatalln(srv.Serve(lis))
}
