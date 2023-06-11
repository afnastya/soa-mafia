package messenger

import (
	"context"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s. Error: %s", msg, err)
		panic(fmt.Sprintf("%s. Error: %s", msg, err))
	}
}

func logOnError(err error, msg string) {
	if err != nil {
		log.Printf("%s. Error: %s", msg, err)
	}
}

type Messenger struct {
	rabbitmq_url string
	chatName     string
	sentMsg      chan string
	receivedMsg  chan string
}

func NewMessenger(rabbitmq_url string, chatName string) *Messenger {
	messenger := Messenger{
		rabbitmq_url,
		chatName,
		make(chan string),
		make(chan string),
	}

	go messenger.publish()
	go messenger.consume()

	return &messenger
}

func (m *Messenger) Send(msg string) {
	m.sentMsg <- msg
}

func (m *Messenger) Receive() chan string {
	return m.receivedMsg
}

func (m *Messenger) Close() {
	close(m.sentMsg)
	close(m.receivedMsg)
}

func (m *Messenger) publish() {
	conn, ch := m.connect()
	defer conn.Close()
	defer ch.Close()

	declareExchange(ch, m.chatName)

	for msg := range m.sentMsg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := ch.PublishWithContext(ctx,
			m.chatName, // exchange
			"",         // routing key
			false,      // mandatory
			false,      // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(msg),
			})

		logOnError(err, "Failed to publish a message: "+msg)
		// log.Printf(" [x] Sent %s", msg)
	}
}

func (m *Messenger) consume() {
	conn, ch := m.connect()
	defer conn.Close()
	defer ch.Close()

	queue := makeQueue(ch, m.chatName)

	msgs, err := ch.Consume(
		queue.Name, // queue
		"",         // consumer
		true,       // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	failOnError(err, "Failed to register a consumer")

	for msg := range msgs {
		m.receivedMsg <- string(msg.Body)
		log.Printf("Received a message: %s", msg.Body)
	}
}

func (m *Messenger) connect() (*amqp.Connection, *amqp.Channel) {
	_, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	conn, err := amqp.Dial(m.rabbitmq_url)
	for err != nil {
		conn, err = amqp.Dial(m.rabbitmq_url)
		time.Sleep(5 * time.Second)
	}
	log.Println("Connected to rabbitmq")
	// failOnError(err, "Failed to connect to RabbitMQ")

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
	}
	failOnError(err, "Failed to open a channel")

	return conn, ch
}

func declareExchange(rabbitmq_channel *amqp.Channel, exchange string) {
	err := rabbitmq_channel.ExchangeDeclare(
		exchange, // name
		"fanout", // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	failOnError(err, "Failed to declare an exchange")
}

func makeQueue(rabbitmq_channel *amqp.Channel, exchange string) *amqp.Queue {
	declareExchange(rabbitmq_channel, exchange)

	queue, err := rabbitmq_channel.QueueDeclare(
		"",    // name
		false, // durable
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
	failOnError(err, "Failed to declare a queue")

	err = rabbitmq_channel.QueueBind(
		queue.Name, // queue name
		"",         // routing key
		exchange,   // exchange
		false,
		nil,
	)
	failOnError(err, "Failed to bind a queue")

	return &queue
}

// func main() {
// 	rabbitmq_url := "amqp://guest:guest@localhost:5672/%2f"
// 	var chatName string
// 	fmt.Scanf("%s", &chatName)

// 	messenger := NewMessenger(rabbitmq_url, chatName)

// 	log.Println("Ready to get messages!")
// 	go func() {
// 		for msg := range messenger.Receive() {
// 			fmt.Println(msg)
// 		}
// 	}()

// 	reader := bufio.NewReader(os.Stdin)
// 	for {
// 		input, err := reader.ReadString('\n')
// 		if err != nil {
// 			fmt.Println("Error reading input:", err)
// 			continue
// 		}

// 		input = strings.TrimSpace(input)
// 		messenger.Send(input)
// 	}
// }
