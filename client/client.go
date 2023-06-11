package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"

	// "log"
	"os"
	"regexp"
	messenger "soa_mafia/messenger"
	"strings"
	"time"

	"soa_mafia/pkg/mafia_grpc"
)

type MafiaClient struct {
	grpc *mafia_grpc.MafiaClient

	name       *string
	playerInfo *mafia_grpc.PlayerInfo
	messenger  *messenger.MafiaMessenger

	stdout *ThreadSafeStdout
}

func NewMafiaClient(grpc_client mafia_grpc.MafiaClient) *MafiaClient {
	return &MafiaClient{
		&grpc_client,
		nil,
		nil,
		nil,
		NewThreadSafeStdout(),
	}
}

// func main() {
// 	RunConsoleClient()
// }

func (m *MafiaClient) Run() {
	m.RunConsoleClient()
}

func (m *MafiaClient) setName(reader *bufio.Reader) {
	for m.name == nil {
		m.stdout.Println("Enter your name:")
		name, err := reader.ReadString('\n')
		name = strings.TrimSpace(name)
		if err != nil {
			m.stdout.Println("Error reading input: " + err.Error())
			continue
		}

		matches, err := regexp.MatchString("^[a-zA-Z0-9]+$", name)
		if name == "" || err != nil || !matches {
			m.stdout.Println("There are invalid symbols in your name. Try again")
			continue
		}

		m.name = &name
	}
}

func parseCmd(line string) (string, string) {
	words := strings.Fields(line)

	if len(words) == 0 {
		return "", ""
	}

	cmd := words[0]
	arg := strings.Join(words[1:], " ")

	return cmd, arg
}

func (m *MafiaClient) RunConsoleClient() {
	reader := bufio.NewReader(os.Stdin)

	m.setName(reader)

	for {
		m.stdout.Println("Enter a command: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			m.stdout.Println("Error reading input: " + err.Error())
			continue
		}

		input = strings.TrimSpace(input)
		cmd, arg := parseCmd(input)

		switch cmd {
		case "help":
			m.help()
		case "join":
			m.newGame(arg)
		case "vote":
			m.vote(arg)
		case "kill":
			m.kill(arg)
		case "check":
			m.check(arg)
		case "state":
			m.state()
		case "msg":
			m.sendMsg(arg)
		case "quit":
			m.quit()
		case "exit":
			m.quit()
			return
		default:
			m.stdout.Println("Invalid command. Please try again or type \"help\" for help")
		}
	}
}

// TODO: add check
func (m *MafiaClient) help() {
	var sb strings.Builder

	sb.WriteString("========================== Available commands ========================\n")
	sb.WriteString("help\n")
	sb.WriteString("join {session_name} \t\t\t join the game\n")
	sb.WriteString("vote {player_name} \t\t\t vote for a player during the day\n")
	sb.WriteString("kill {player_name} \t\t\t kill a player. (command only for mafia)\n")
	sb.WriteString("check {player_name} \t\t\t check if player is mafia. (command only for detective)\n")
	sb.WriteString("state \t\t\t\t\t get current game state\n")
	sb.WriteString("msg {text} \t\t\t\t send text to chat\n")
	sb.WriteString("quit \t\t\t\t\t quit the game session\n")
	sb.WriteString("exit \t\t\t\t\t exit the program\n")

	m.stdout.Println(sb.String())
}

func stateToString(state *mafia_grpc.GameState) string {
	var sb strings.Builder

	sb.WriteString("===================== Game State =====================\n")
	sb.WriteString(fmt.Sprintf("Session: %s\n", state.Session))

	sb.WriteString("Alive Players:\n")
	for _, player := range state.AlivePlayers {
		sb.WriteString(fmt.Sprintf("- %s\n", player))
	}

	sb.WriteString(fmt.Sprintf("Date: %d\n", state.Date))
	sb.WriteString("Time of the day: ")
	if state.IsDay {
		sb.WriteString("Day\n")
	} else {
		sb.WriteString("Night\n")
	}

	gameStatus := "not started"
	if state.IsStarted {
		gameStatus = "started"
	}

	if state.IsFinished {
		gameStatus = "finished"
	}

	sb.WriteString(fmt.Sprintf("Game %s\n", gameStatus))

	return sb.String()
}

func (m *MafiaClient) printErrorIfNotPlaying() bool {
	if m.playerInfo == nil {
		m.stdout.Println("You haven't joined any sessions yet")
		return true
	}

	return false
}

func (m *MafiaClient) state() {
	if m.printErrorIfNotPlaying() {
		return
	}

	state, err := m.getStateGrpc()
	if err != nil {
		m.stdout.Println("Error while requesting the state of the game " + m.playerInfo.Session + ": " + err.Error())
		return
	}

	m.stdout.Println(stateToString(state))
}

func (m *MafiaClient) sendMsg(msg string) {
	if m.printErrorIfNotPlaying() || m.messenger == nil {
		return
	}

	err := m.messenger.Send(msg)
	if err != nil {
		m.stdout.Println("Error while sending a message: " + err.Error())
	}
}

func containsString(slice []string, str string) bool {
	for _, value := range slice {
		if value == str {
			return true
		}
	}
	return false
}

func notificationToString(notification *mafia_grpc.Notification) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Notification: %s\n", notification.Type))
	if notification.Type == mafia_grpc.NotificationType_START {
		sb.WriteString(fmt.Sprintf("Your role: %s", notification.GetRole()))
	} else if notification.Type == mafia_grpc.NotificationType_FINISH {
		sb.WriteString(fmt.Sprintf("Mafia %s ", notification.GetMafia()))
		alive := notification.GameState.AlivePlayers
		mafiaWon := containsString(alive, notification.GetMafia())

		if mafiaWon {
			sb.WriteString("won!")
		} else {
			sb.WriteString("lost!")
		}
	} else {
		killed := notification.GetKilledPlayer()
		if killed == "" {
			killed = "Nobody"
		}
		sb.WriteString(fmt.Sprintf("%s was killed!", killed))
	}
	return sb.String()
}

func (m *MafiaClient) processNotifications(notifications chan *mafia_grpc.Notification) {
	for notification := range notifications {
		// log.Println(notificationToString(notification))
		m.stdout.Println(notificationToString(notification))
	}
	// log.Println("processNotifications ended")
}

func (m *MafiaClient) processMessages() {
	for msg := range m.messenger.Receive() {
		m.stdout.Println(msg)
	}
	// log.Println("processMessages ended")
}

func (m *MafiaClient) newGame(session string) {
	if m.playerInfo != nil {
		m.quit()
	}

	_, err := m.joinGrpc(session)
	if err != nil {
		m.stdout.Println("Error while joining the game session " + session + ": " + err.Error())
		return
	}

	m.playerInfo = &mafia_grpc.PlayerInfo{Session: session, Name: *m.name}

	notifications, err := m.getNotificationsGrpc()
	if err != nil {
		m.stdout.Println("Error while trying to subscribe for notifications: " + err.Error())
		return
	}

	go m.processNotifications(notifications)
	m.messenger = messenger.NewMafiaMessenger(m.grpc, m.playerInfo)
	go m.processMessages()
}

func (m *MafiaClient) quit() {
	if m.printErrorIfNotPlaying() {
		return
	}

	_, err := m.quitGrpc()
	if err != nil {
		m.stdout.Println("Error while quiting the game: " + err.Error())
	}

	m.messenger.Close()
	m.messenger = nil
	m.playerInfo = nil
}

func (m *MafiaClient) vote(victim string) {
	if m.printErrorIfNotPlaying() {
		return
	}

	_, err := m.voteGrpc(victim)
	if err != nil {
		m.stdout.Println("Error while voting: " + err.Error())
		return
	}
}

func (m *MafiaClient) kill(victim string) {
	if m.printErrorIfNotPlaying() {
		return
	}

	_, err := m.killGrpc(victim)
	if err != nil {
		m.stdout.Println("Error while killing: " + err.Error())
		return
	}
}

func (m *MafiaClient) check(victim string) {
	if m.printErrorIfNotPlaying() {
		return
	}

	resp, err := m.checkIfMafiaGrpc(victim)
	if err != nil {
		m.stdout.Println("Error while checking if mafia: " + err.Error())
		return
	}

	if resp.IsMafia {
		m.stdout.Println(fmt.Sprintf("%s is mafia", victim))
	} else {
		m.stdout.Println(fmt.Sprintf("%s is not mafia", victim))
	}
}

//////////////////////////////////////////// Private methods: grpc calls //////////////////////////////////////////

func (m *MafiaClient) joinGrpc(session string) (*mafia_grpc.Response, error) {
	if m.playerInfo != nil {
		return &mafia_grpc.Response{}, errors.New("Already joined the game")
	}

	playerInfo := &mafia_grpc.PlayerInfo{Session: session, Name: *m.name}

	request := &mafia_grpc.JoinRequest{Player: playerInfo}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := (*m.grpc).Join(ctx, request)
	return response, err
}

func (m *MafiaClient) voteGrpc(victim string) (*mafia_grpc.Response, error) {
	request := &mafia_grpc.SetVictimRequest{Player: m.playerInfo, Victim: victim}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := (*m.grpc).Vote(ctx, request)
	return response, err
}

func (m *MafiaClient) killGrpc(victim string) (*mafia_grpc.Response, error) {
	request := &mafia_grpc.SetVictimRequest{Player: m.playerInfo, Victim: victim}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := (*m.grpc).Kill(ctx, request)
	return response, err
}

func (m *MafiaClient) checkIfMafiaGrpc(victim string) (*mafia_grpc.CheckMafiaResponse, error) {
	request := &mafia_grpc.SetVictimRequest{Player: m.playerInfo, Victim: victim}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := (*m.grpc).CheckIfMafia(ctx, request)
	return response, err
}

func (m *MafiaClient) getStateGrpc() (*mafia_grpc.GameState, error) {
	request := m.playerInfo
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := (*m.grpc).GetState(ctx, request)
	return response, err
}

func (m *MafiaClient) quitGrpc() (*mafia_grpc.Response, error) {
	request := m.playerInfo
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := (*m.grpc).Quit(ctx, request)
	return response, err
}

func (m *MafiaClient) getNotificationsGrpc() (chan *mafia_grpc.Notification, error) {
	request := m.playerInfo

	notifications := make(chan *mafia_grpc.Notification)

	notification_stream, err := (*m.grpc).GetNotifications(context.Background(), request)

	go func() {
		for {
			notification, err := notification_stream.Recv()
			if err != nil {
				// log.Println(err)
				break
			}
			// log.Println(notificationToString(notification))
			notifications <- notification
		}
		// log.Println("Notification stream closed")
	}()

	return notifications, err
}
