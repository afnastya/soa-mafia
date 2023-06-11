package main

import (
	"fmt"
	"log"
	"os"

	"soa_mafia/pkg/mafia_grpc"

	"google.golang.org/grpc"
)

func main() {
	log.Println("Client running ...")
	mafia_host := os.Getenv("MAFIA_HOST")
	conn, err := grpc.Dial(fmt.Sprintf("%s:9000", mafia_host), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	grpc_client := mafia_grpc.NewMafiaClient(conn)

	mafia_client := NewMafiaClient(grpc_client)

	mafia_client.Run()
	// request := &mafia_grpc.JoinRequest{Player: &player_info}
	// ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	// defer cancel()

	// response, err := client.Join(ctx, request)
	// if err != nil {
	// 	log.Fatalln(err)
	// }

	// log.Println("Response:", response.GetOk())
}
