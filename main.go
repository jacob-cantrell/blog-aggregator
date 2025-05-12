package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jacob-cantrell/blog-aggregator/internal/config"
	"github.com/jacob-cantrell/blog-aggregator/internal/database"
	_ "github.com/lib/pq"
)

type state struct {
	db  *database.Queries
	cfg *config.Config
}

type commands struct {
	com map[string]func(*state, command) error
}

type command struct {
	name string
	args []string
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.com[name] = f
}

func (c *commands) run(s *state, cmd command) error {
	if c.com[cmd.name] == nil {
		return errors.New("command does not exist")
	}

	if err := c.com[cmd.name](s, cmd); err != nil {
		return err
	}
	return nil
}

func handlerLogin(s *state, cmd command) error {
	// Need at least one argument
	if len(cmd.args) == 0 {
		return errors.New("username required with login command")
	}

	// Check if user exists
	_, err := s.db.GetUser(context.Background(), cmd.args[0])
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// User doesn't exist
			fmt.Println("User doesn't exist!")
			os.Exit(1)
		}
		// Some other database error
		return err
	}

	// If we get here, user exists and we can proceed with login

	// Set username, catch error from SetUser and return if not nil
	if err := s.cfg.SetUser(cmd.args[0]); err != nil {
		return err
	}

	// Print that user has been set, return nil
	fmt.Println("User has been set!")
	return nil
}

func handlerRegister(s *state, cmd command) error {
	// Make sure at least one argument
	if len(cmd.args) == 0 {
		return errors.New("name required with register command")
	}

	name := cmd.args[0]

	// Check if user already exists
	_, err := s.db.GetUser(context.Background(), name)
	if err == nil {
		if !errors.Is(err, sql.ErrNoRows) {
			// User doesn't exist
			fmt.Println("User already exists!")
			os.Exit(1)
		}
		// Some other database error
		return err
	}

	// Create new user in database
	params := database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      name,
	}

	user, err := s.db.CreateUser(context.Background(), params)
	if err != nil {
		return err
	}

	// Update config with current user
	s.cfg.SetUser(user.Name)

	// Print success message
	fmt.Println("User successfully created!")

	return nil
}

func handlerReset(s *state, cmd command) error {
	err := s.db.Reset(context.Background())
	if err != nil {
		return err
	}

	return nil
}

func handlerUsers(s *state, cmd command) error {
	// Get users
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return err
	}

	// If length is 0, then no users exist
	if len(users) == 0 {
		fmt.Println("No users in database!")
		os.Exit(1)
	}

	// Loop through users, print information
	for i := 0; i < len(users); i++ {
		if users[i].Name == s.cfg.CurrUsername {
			fmt.Printf("* %s (current)\n", users[i].Name)
		} else {
			fmt.Printf("* %s\n", users[i].Name)
		}
	}

	return nil
}

func main() {
	// Create config from ~/.gatorconfig.json
	c, err := config.Read()
	if err != nil {
		log.Fatal(err)
	}

	// Create state & apply config; register commands to commands struct
	s := &state{}
	s.cfg = &c
	coms := commands{com: make(map[string]func(*state, command) error)}
	coms.register("login", handlerLogin)
	coms.register("register", handlerRegister)
	coms.register("reset", handlerReset)
	coms.register("users", handlerUsers)

	// Open connection to database
	db, err := sql.Open("postgres", c.DBUrl)
	if err != nil {
		log.Fatal(err)
	}

	// Create & store db queries into state struct
	dbQueries := database.New(db)
	s.db = dbQueries

	// Read arguments, check for errors
	args := os.Args
	if len(args) < 2 {
		fmt.Println("less than 2 arguments is not allowed")
		os.Exit(1)
	}

	// Create command to run
	com := command{name: string(args[1]), args: []string(args[2:])}

	// Run command & check for errors
	if err := coms.run(s, com); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
