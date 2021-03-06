package main

import (
	"bufio"
	"encoding/json"
	zmq "github.com/pebbe/zmq4"
	"github.com/prometheus/client_golang/prometheus"
	serial "github.com/tarm/goserial"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type AIR struct {
	site    string
	device  string
	address string
}

type Message struct {
	Pressure    float64   `json:"pressure"`
	Temperature float64   `json:"temperature"`
	Vol         float64   `json:"volumetic-flow"`
	Mass        float64   `json:"mass-flow"`
	Setpoint    float64   `json:"set-point"`
	Gas         string    `json:"gas"`
	Site        string    `json:"site"`
	At          time.Time `json:"at"`
}

var (
	airLog = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "trailer_air_flow",
		Help: "Current Air flow.",
	})
)

func (air AIR) Sample() string {

	data := air.read()
	message := air.parse(data)
	airLog.Set(message.Mass)
	json_message, err := json.Marshal(message)
	if err != nil {
		log.Fatal(err)
	}
	return string(json_message)
}

func (air AIR) parse(data string) Message {
	/* log.Print(data) */
	elements := strings.Fields(data)

	/* log.Print(elements) */
	pressure, _ := strconv.ParseFloat(elements[1], 64)
	temperature, _ := strconv.ParseFloat(elements[2], 64)
	vol, _ := strconv.ParseFloat(elements[3], 64)
	mass, _ := strconv.ParseFloat(elements[4], 64)
	setpoint, _ := strconv.ParseFloat(elements[5], 64)
	gas := elements[6]

	return Message{pressure, temperature, vol, mass, setpoint, gas, air.site, time.Now()}
}

// Send a couple of returns to wake up the controller
func (air AIR) wake() {
	c := serial.Config{Name: air.device, Baud: 9600}
	port, err := serial.OpenPort(&c)
	if err != nil {
		log.Fatal(err)
	}
	defer port.Close()

	for i := 0; i < 5; i++ {
		query := air.address + "\r"
		_, err = port.Write([]byte(query))
		if err != nil {
			log.Fatal(err)
		}
	}
}

func (air AIR) read() string {
	c := serial.Config{Name: air.device, Baud: 9600}
	port, err := serial.OpenPort(&c)
	if err != nil {
		log.Fatal(err)
	}
	defer port.Close()

	query := air.address + "\r"
	_, err = port.Write([]byte(query))
	if err != nil {
		log.Fatal(err)
	}

	reader := bufio.NewReader(port)
	reply, err := reader.ReadBytes('\x0d')
	if err != nil {
		log.Fatal(err)
	}

	return string(reply)
}

func readMassFlowController() {
	air := AIR{}
	air.site = "glbrc"
	air.device = "/dev/ttyS5"
	air.address = "A"

	socket, err := zmq.NewSocket(zmq.PUB)
	if err != nil {
		log.Fatal(err)
	}
	defer socket.Close()
	socket.Bind("tcp://*:5558")

	air.wake()

	for {
		sample := air.Sample()
		log.Print(sample)
		socket.Send(sample, 0)
		time.Sleep(10 * time.Second)
	}
}

func init() {
	prometheus.MustRegister(airLog)
}

func main() {
	go readMassFlowController()
	http.Handle("/metrics", prometheus.Handler())
	http.ListenAndServe(":9093", nil)
}
