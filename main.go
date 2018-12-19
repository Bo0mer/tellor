package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/nlopes/slack"
	"gobot.io/x/gobot"
	"gobot.io/x/gobot/platforms/dji/tello"
)

// Command instructs a drone to do something.
type Command string

const (
	CommandMoveUp                 Command = "up"
	CommandMoveDown                       = "down"
	CommandMoveLeft                       = "left"
	CommandMoveRight                      = "rigth"
	CommandMoveForward                    = "forward"
	CommandMoveBackward                   = "backward"
	CommandRotateClockwise                = "clockwise"
	CommandRotateCounterClockwise         = "counter_clockwise"
	CommandFlip                           = "flip"
	CommandFrontFlip                      = "front_flip"
	CommandRightFlip                      = "right_flip"
	CommandHover                          = "hover"
)

func main() {
	slacker := &slacker{Token: os.Getenv("SLACK_TOKEN")}
	drone := &drone{Port: os.Getenv("DRONE_PORT")}
	drone.Start()

	inCommands := slacker.CommandChan()
	outCommands := drone.CommandChan()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case c := <-inCommands:
			outCommands <- c
		case <-sigChan:
			goto shutdown
		}
	}

shutdown:
	if err := drone.Stop(); err != nil {
		log.Fatalf("drone shutdown failed: %v\n", err)
	}
}

type status struct {
	ready bool
	sync.Mutex
}

type drone struct {
	Port string

	driver   *tello.Driver
	commands chan Command
	status   status
}

// Start starts the drone.
func (d *drone) Start() {
	d.driver = tello.NewDriver(d.Port)

	work := func() {
		d.driver.On(tello.ConnectedEvent, func(data interface{}) {
			d.driver.TakeOff()

			d.status.Lock()
			d.status.ready = true
			d.status.Unlock()
		})
	}
	robot := gobot.NewRobot("tello",
		[]gobot.Connection{},
		[]gobot.Device{d.driver},
		work,
	)

	go robot.Start()
}

// Stop stops the drone.
func (d *drone) Stop() error {
	if d.driver != nil {
		return d.driver.Land()
	}
	return errors.New("drone not started")
}

func (d *drone) CommandChan() chan<- Command {
	d.commands = make(chan Command)
	go d.handleCommands()
	return d.commands
}

func (d *drone) handleCommands() {
	for c := range d.commands {
		fmt.Printf("executing command %v\n", c)
		switch c {
		case CommandMoveUp:
			d.driver.Up(10)
		case CommandMoveDown:
			d.driver.Down(10)
		case CommandMoveLeft:
			d.driver.Left(10)
		case CommandMoveRight:
			d.driver.Right(10)
		case CommandMoveForward:
			d.driver.Forward(10)
		case CommandMoveBackward:
			d.driver.Backward(10)
		case CommandHover:
			d.driver.Hover()
		case CommandRotateClockwise:
			d.driver.Clockwise(10)
		case CommandFrontFlip:
			d.driver.FrontFlip()
		case CommandRightFlip:
			d.driver.RightFlip()
		case CommandRotateCounterClockwise:
			d.driver.CounterClockwise(10)
		}
	}
}

type slacker struct {
	Token    string
	commands chan Command
}

func (s *slacker) CommandChan() <-chan Command {
	rtm := slack.New(s.Token).NewRTM()
	go rtm.ManageConnection()

	go func() {
		for msg := range rtm.IncomingEvents {
			switch ev := msg.Data.(type) {
			case *slack.MessageEvent:
				s.handleMessageEvent(ev)
			}
		}
	}()

	s.commands = make(chan Command)
	return s.commands
}

func (s *slacker) handleMessageEvent(m *slack.MessageEvent) {
	fmt.Printf("Handling message %q\n", m.Msg.Text)
	switch strings.ToLower(m.Msg.Text) {
	case "up":
		s.commands <- CommandMoveUp
	case "down":
		s.commands <- CommandMoveDown
	case "left":
		s.commands <- CommandMoveLeft
	case "right":
		s.commands <- CommandMoveRight
	case "forward":
		s.commands <- CommandMoveForward
	case "backward":
		s.commands <- CommandMoveBackward
	case "rotate", "rotate clockwise":
		s.commands <- CommandRotateClockwise
	case "rotate cc":
		s.commands <- CommandRotateCounterClockwise
	case "flip":
		s.commands <- CommandFlip
	case "right flip":
		s.commands <- CommandRightFlip
	case "front flip":
		s.commands <- CommandFrontFlip
	case "halt", "hover", "steady":
		s.commands <- CommandHover
	}
}
