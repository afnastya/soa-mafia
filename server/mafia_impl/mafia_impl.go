package mafia_impl

import (
	// "context"
	// "fmt"
	"errors"
	"log"
	"math/rand"
	"soa_mafia/pkg/mafia_grpc"
	"strings"
	"time"
	// "google.golang.org/grpc"
)

/*
	TODO:
		- rwlock
		- notify through channels
		- pretty print, logging
		- custom errors with constants and messages
*/

const (
	MaxPlayers int32 = 4
)

type Role string

const (
	Mafia     Role = "Mafia"
	Detective Role = "Detective"
	Civilian  Role = "Civilian"
)

type playerInfo struct {
	role          Role
	isAlive       bool
	hasVoted      bool
	notifications *chan *mafia_grpc.Notification
}

type Game struct {
	session       string
	names2players map[string]playerInfo
	isStarted     bool
	isFinished    bool

	alivePlayers int32
	date         int32

	isDay            bool
	names2votes      map[string]int32
	votes            int32
	mafiaChoice      string
	detectiveChecked bool
}

////////////////////////////////////////////////// API ///////////////////////////////////////////////////////

func NewGame(session string) *Game {
	game := Game{}
	game.init(session)
	return &game
}

func (g *Game) AddPlayer(name string) error {
	if g.isStarted {
		return errors.New("No more players can be added")
	}

	if name == "" {
		return errors.New("Name is empty")
	}

	_, exist := g.names2players[name]
	if exist {
		return errors.New("Player already exists")
	}

	notifications := make(chan *mafia_grpc.Notification)
	g.names2players[name] = playerInfo{
		isAlive:       true,
		hasVoted:      false,
		notifications: &notifications,
	}

	g.alivePlayers++
	if g.alivePlayers == MaxPlayers {
		go g.start()
	}

	return nil
}

func (g *Game) AddVote(player string, victim string) error {
	log.Println("AddVote: ", player, " -> ", victim)

	if g.isFinished {
		return errors.New("Game has finished already")
	}

	if !g.isStarted {
		return errors.New("Game hasn't started yet")
	}

	if !g.isDay {
		return errors.New("Players can't vote during night")
	}

	if !g.canVote(player) {
		return errors.New("Player can't vote or doesn't exist")
	}

	if !g.canBeKilled(victim) {
		return errors.New("Victim can't be killed or doesn't exist")
	}

	g.names2votes[victim]++
	g.votes++

	pInfo := g.names2players[player]
	pInfo.hasVoted = true
	g.names2players[player] = pInfo

	g.continueIfPossible()
	return nil
}

func (g *Game) KillPlayer(mafia string, victim string) error {
	log.Println("KillPlayer: ", mafia, " -> ", victim)

	if g.isFinished {
		return errors.New("Game has finished already")
	}

	if !g.isStarted {
		return errors.New("Game hasn't started yet")
	}

	if g.isDay {
		return errors.New("Mafia can't kill during day")
	}

	if !g.canKill(mafia) {
		return errors.New("Player is not mafia or doesn't exists")
	}

	if !g.canBeKilled(victim) {
		return errors.New("Victim can't be killed or doesn't exist")
	}

	g.mafiaChoice = victim

	g.continueIfPossible()
	return nil
}

func (g *Game) CheckIfMafia(detective string, suggestedMafia string) (bool, error) {
	log.Println("CheckIfMafia: ", detective, " -> ", suggestedMafia)

	if g.isFinished {
		return false, errors.New("Game has finished already")
	}

	if !g.isStarted {
		return false, errors.New("Game hasn't started yet")
	}

	if g.isDay {
		return false, errors.New("Detective can't check during the day")
	}

	if !g.canCheck(detective) {
		return false, errors.New("Player isn't detective or doesn't exist")
	}

	if !g.canBeChecked(suggestedMafia) {
		return false, errors.New("Suggested Mafia is already dead or doesn't exist")
	}

	g.detectiveChecked = true
	g.continueIfPossible()
	return g.names2players[suggestedMafia].role == Mafia, nil
}

func (g *Game) GetNotifications(player string) (*chan *mafia_grpc.Notification, error) {
	info, exists := g.names2players[player]
	if !exists {
		return nil, errors.New("Player doesn't exist")
	}

	return info.notifications, nil
}

func (g *Game) CanChat(player string) (bool, error) {
	info, exists := g.names2players[player]
	if !exists {
		return false, errors.New("Player doesn't exist")
	}

	canChat := info.isAlive && g.isDay && g.isStarted && !g.isFinished

	return canChat, nil
}

func (g *Game) DeletePlayer(player string) error {
	info, exists := g.names2players[player]
	if !exists {
		return errors.New("Player doesn't exist")
	}

	if info.isAlive {
		g.kill(player)
	}

	info = g.names2players[player]
	if info.notifications != nil {
		close(*info.notifications)
		info.notifications = nil
		g.names2players[player] = info
	}

	g.checkIfFinished()
	return nil
}

/////////////////////////////////////////////// checkers ////////////////////////////////////////////////////

func (g *Game) canCheck(detective string) bool {
	info, exists := g.names2players[detective]
	return exists && info.isAlive && info.role == Detective && !g.detectiveChecked
}

func (g *Game) canBeChecked(suggestedMafia string) bool {
	info, exists := g.names2players[suggestedMafia]
	return exists && info.isAlive
}

func (g *Game) isDayOver() bool {
	if !g.isDay {
		return false
	}

	return g.votes >= g.alivePlayers
}

func (g *Game) isNightOver() bool {
	if g.isDay {
		return false
	}

	detective := g.getDetective()
	detectiveInfo := g.names2players[detective]
	detectiveChecked := !detectiveInfo.isAlive || g.detectiveChecked

	return g.mafiaChoice != "" && detectiveChecked
}

//////////////////////////////////////////

func (g *Game) continueIfPossible() {
	if !g.isDayOver() && !g.isNightOver() {
		return
	}

	var victim string
	if g.isDay {
		victim = g.determineDailyVictim()
	} else {
		victim = g.mafiaChoice
	}

	if victim != "" {
		g.kill(victim)
	}

	if g.isDay {
		if g.checkIfFinished() {
			return
		}
		g.newNight()
		g.notifyAll(g.getNotification(mafia_grpc.NotificationType_NEW_NIGHT, &victim))
	} else {
		if g.checkIfFinished() {
			return
		}
		g.newDay()
		g.notifyAll(g.getNotification(mafia_grpc.NotificationType_NEW_DAY, &victim))
	}
}

func (g *Game) checkIfFinished() bool {
	if g.isFinished {
		return true
	}

	mafia := g.getMafia()
	info := g.names2players[mafia]
	if !info.isAlive {
		g.isFinished = true
		g.notifyAll(g.getNotification(mafia_grpc.NotificationType_FINISH, nil))
		return true
	}

	if g.alivePlayers > 1 {
		return false
	}

	g.isFinished = true
	g.notifyAll(g.getNotification(mafia_grpc.NotificationType_FINISH, nil))
	return true
}

func (g *Game) canVote(player string) bool {
	info, exists := g.names2players[player]
	return exists && info.isAlive && !info.hasVoted
}

func (g *Game) canKill(mafia string) bool {
	info, exists := g.names2players[mafia]
	return exists && info.isAlive && info.role == Mafia && g.mafiaChoice == ""
}

func (g *Game) canBeKilled(victim string) bool {
	info, exists := g.names2players[victim]
	return exists && info.isAlive
}

//////////////////////////////////////////// Private methods ///////////////////////////////////////////////

func (g *Game) init(session string) {
	g.session = session
	g.names2players = make(map[string]playerInfo)
	g.isStarted = false
	g.isFinished = false

	g.alivePlayers = 0
	g.date = 0

	g.isDay = true
	g.names2votes = make(map[string]int32)
	g.votes = 0
	g.mafiaChoice = ""
	g.detectiveChecked = false
}

func (g *Game) generateRoles() {
	roles := []Role{Mafia, Detective, Civilian, Civilian}

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(roles), func(i, j int) {
		roles[i], roles[j] = roles[j], roles[i]
	})

	for name := range g.names2players {
		player := g.names2players[name]
		player.role = roles[0]
		g.names2players[name] = player

		roles = roles[1:]
	}

	log.Printf("Session %s: Roles assigned successfully!", g.session)
}

func (g *Game) start() {
	g.generateRoles()
	g.isStarted = true
	g.newNight()
	g.notifyStart()
}

func (g *Game) newDay() {
	g.isDay = true

	g.names2votes = make(map[string]int32)
	g.votes = 0
}

func (g *Game) newNight() {
	g.date++
	g.isDay = false
	g.mafiaChoice = ""
	g.detectiveChecked = false
}

/*
	Method is called only if all required checks are made
*/
func (g *Game) kill(victim string) {
	vInfo := g.names2players[victim]
	vInfo.isAlive = false
	g.names2players[victim] = vInfo

	g.alivePlayers--
}

/*
	Method must be called only after all players have voted.
	Returns name of dailyVictim. If votes split, then returns ""
*/
func (g *Game) determineDailyVictim() string {
	var maxValue int32 = 0
	dailyVictim := ""
	isSingle := true
	for name, value := range g.names2votes {
		if value > maxValue {
			maxValue = value
			dailyVictim = name
			isSingle = true
		} else if value == maxValue {
			isSingle = false
		}
	}

	if isSingle {
		return dailyVictim
	}

	return ""
}

func (g *Game) notifyStart() {
	for _, info := range g.names2players {
		notification := g.getNotification(mafia_grpc.NotificationType_START, (*string)(&info.role))
		if info.notifications != nil {
			*info.notifications <- notification
		}
	}
}

func (g *Game) notifyAll(notification *mafia_grpc.Notification) {
	for _, info := range g.names2players {
		log.Println("Notification: ", notification.Type)
		if info.notifications != nil {
			*info.notifications <- notification
		}
	}
}

///////////////////////////////////////////// getters //////////////////////////////////////////////////

func (g *Game) getMafia() string {
	for name, info := range g.names2players {
		if info.role == Mafia {
			return name
		}
	}

	return ""
}

func (g *Game) getDetective() string {
	for name, info := range g.names2players {
		if info.role == Detective {
			return name
		}
	}

	return ""
}

func (g *Game) GetAlivePlayers() []string {
	alivePlayers := []string{}
	for name, info := range g.names2players {
		if info.isAlive {
			alivePlayers = append(alivePlayers, name)
		}
	}

	return alivePlayers
}

func (g *Game) GetGameState() *mafia_grpc.GameState {
	return &mafia_grpc.GameState{
		Session:      g.session,
		AlivePlayers: g.GetAlivePlayers(),
		Date:         g.date,
		IsDay:        g.isDay,
		IsStarted:    g.isStarted,
		IsFinished:   g.isFinished,
	}
}

func (g *Game) getNotification(nType mafia_grpc.NotificationType, detail *string) *mafia_grpc.Notification {
	state := g.GetGameState()

	switch nType {
	case mafia_grpc.NotificationType_START:
		return &mafia_grpc.Notification{
			Type:      nType,
			GameState: state,
			Details: &mafia_grpc.Notification_Role{
				Role: *detail,
			},
		}
	case mafia_grpc.NotificationType_FINISH:
		return &mafia_grpc.Notification{
			Type:      nType,
			GameState: state,
			Details: &mafia_grpc.Notification_Mafia{
				Mafia: g.getMafia(),
			},
		}
	case mafia_grpc.NotificationType_NEW_DAY:
		return &mafia_grpc.Notification{
			Type:      nType,
			GameState: state,
			Details: &mafia_grpc.Notification_KilledPlayer{
				KilledPlayer: *detail,
			},
		}
	case mafia_grpc.NotificationType_NEW_NIGHT:
		return &mafia_grpc.Notification{
			Type:      nType,
			GameState: state,
			Details: &mafia_grpc.Notification_KilledPlayer{
				KilledPlayer: *detail,
			},
		}
	default:
		return nil
	}
}

////////////////////////////////////////// Pretty print ////////////////////////////////////////////////

func (g *Game) String() string {
	var sb strings.Builder
	sb.WriteString("Game Info:\n")
	sb.WriteString("Session: " + string(g.session) + "\n")
	sb.WriteString(g.Players2String())
	return sb.String()
}

func (g *Game) Players2String() string {
	var sb strings.Builder

	sb.WriteString("Players:\n")
	for name, info := range g.names2players {
		sb.WriteString(name + ": " + string(info.role) + ", ")
		if info.isAlive {
			sb.WriteString("alive\n")
		} else {
			sb.WriteString("dead\n")
		}
	}

	return sb.String()
}

// func assert(condition bool, msg string) {
// 	if !condition {
// 		panic(msg)
// 	}
// }

// func main() {
// 	game := NewGame("12345")

// 	assert(!game.isStarted, "game must not be started yet")

// 	game.AddPlayer("p1")
// 	game.AddPlayer("p1")
// 	game.AddPlayer("p1")
// 	game.AddPlayer("p2")
// 	assert(!game.isStarted, "game must not be started yet")

// 	game.AddPlayer("p3")
// 	game.AddPlayer("p4")
// 	assert(game.isStarted, "game must be started already")

// 	for i := 0; i < 3; i++ {
// 		err := game.AddPlayer("p5")
// 		assert(err != nil, "too many players")
// 	}

// 	//////////////////////////////////////////////////////////////////////////////// Game start ///////////////

// 	mafia := game.getMafia()
// 	detective := game.getDetective()
// 	log.Println("Mafia: ", mafia)
// 	log.Println("Detective: ", detective)

// 	players := []string{"p1", "p2", "p3", "p4"}

// 	for {
// 		log.Println(game.String())

// 		for p_id := range players {
// 			for vote_id := range players {
// 				if p_id == vote_id {
// 					continue
// 				}

// 				err := game.AddVote(players[p_id], players[vote_id])
// 				if err != nil {
// 					log.Println(err)
// 				}
// 			}
// 		}

// 		log.Println(game.String())

// 		alive := game.GetAlivePlayers()
// 		log.Println("Alive Players: ", alive)
// 		rand.Seed(time.Now().UnixNano())
// 		randomVictim := rand.Intn(len(alive))

// 		err := game.KillPlayer(mafia, alive[randomVictim])
// 		if err != nil {
// 			log.Println(err)
// 		}

// 		log.Println(game.String())

// 		randomVictim = rand.Intn(len(alive))
// 		_, err = game.CheckIfMafia(detective, alive[randomVictim])
// 		if err != nil {
// 			log.Println(err)
// 		}

// 		log.Println(game.String())

// 		for p_id := range players {
// 			for victim_id := range players {
// 				if p_id == victim_id {
// 					continue
// 				}

// 				err := game.KillPlayer(players[p_id], players[victim_id])
// 				if err != nil {
// 					log.Println(err)
// 				}
// 			}
// 		}

// 		log.Println(game.String())

// 		if game.isFinished {
// 			break
// 		}

// 		time.Sleep(time.Second)
// 	}
// }
