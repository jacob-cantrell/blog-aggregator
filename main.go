package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
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

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	// 1. Create a new HTTP request with the given context
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, err
	}

	// 2. Set the User-Agent header
	req.Header.Set("User-Agent", "gator")

	// 3. Create an HTTP client and send the request
	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// 4. Read the response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	// 5. Unmarshal the XML
	rss := RSSFeed{}
	err = xml.Unmarshal(body, &rss)
	if err != nil {
		return nil, err
	}

	// 6. Unescape HTML entities
	rss.Channel.Title = html.UnescapeString(rss.Channel.Title)
	rss.Channel.Description = html.UnescapeString(rss.Channel.Description)

	for i := range rss.Channel.Item {
		rss.Channel.Item[i].Title = html.UnescapeString(rss.Channel.Item[i].Title)
		rss.Channel.Item[i].Description = html.UnescapeString(rss.Channel.Item[i].Description)
	}

	// 7. Return the feed
	return &rss, nil
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		u, err := s.db.GetUser(context.Background(), s.cfg.CurrUsername)
		if err != nil {
			return err
		}

		return handler(s, cmd, u)
	}
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

func handlerAddFeed(s *state, cmd command, user database.User) error {
	// Check for 2 arguments
	if len(cmd.args) != 2 {
		fmt.Println("2 arguments (name & url) required for 'addfeed' command!")
		os.Exit(1)
	}

	// Create params
	params := database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.args[0],
		Url:       cmd.args[1],
		UserID:    user.ID,
	}

	// Create feed
	feed, err := s.db.CreateFeed(context.Background(), params)
	if err != nil {
		return err
	}

	// Create feed_follows record
	followParam := database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user.ID,
		FeedID:    feed.ID,
	}

	_, err1 := s.db.CreateFeedFollow(context.Background(), followParam)
	if err1 != nil {
		return err1
	}

	return nil
}

func handlerAgg(s *state, cmd command) error {
	r, err := fetchFeed(context.Background(), "https://www.wagslane.dev/index.xml")
	if err != nil {
		return err
	}

	// PrettyPrint the RSSFeed struct
	jsonBytes, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonBytes))

	return nil
}

func handlerFeeds(s *state, cmd command) error {
	// Retrieve rows from Feeds table
	f, err := s.db.GetFeeds(context.Background())
	if err != nil {
		return err
	}

	// Check if any feeds in database
	if len(f) == 0 {
		fmt.Println("No feeds available!")
		os.Exit(1)
	}

	// For each feed, print info
	fmt.Println("******** FEEDS ********")
	for i := range f {
		u, err := s.db.GetUserByID(context.Background(), f[i].UserID)
		if err != nil {
			return err
		}
		fmt.Printf("Name: %s\n", f[i].Name)
		fmt.Printf("URL: %s\n", f[i].Url)
		fmt.Printf("User Name: %s\n", u.Name)

		if i != (len(f) - 1) {
			fmt.Println("-------------------------")
		}
	}
	fmt.Println("******** FEEDS ********")

	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	// Verify one argument included
	if len(cmd.args) != 1 {
		return errors.New("URL required with follow command")
	}

	// Get feed by url
	f, err := s.db.GetFeedByURL(context.Background(), cmd.args[0])
	if err != nil {
		return err
	}

	// Create feed-follow parameters
	params := database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user.ID,
		FeedID:    f.ID,
	}

	// Create feed_follows record
	ff, err := s.db.CreateFeedFollow(context.Background(), params)
	if err != nil {
		return err
	}

	// Print record information
	fmt.Println("********** NEW FOLLOW **********")
	fmt.Printf("Feed Name: %s\n", ff.FeedName)
	fmt.Printf("User Name: %s\n", ff.UserName)
	fmt.Println("********** NEW FOLLOW **********")

	return nil
}

func handlerFollowing(s *state, cmd command, user database.User) error {

	// Get feed_follows records by User ID
	follows, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return err
	}

	// Check if any records came back
	if len(follows) == 0 {
		fmt.Printf("%s isn't following anyone!\n", user.Name)
		return nil
	}

	// Loop through follows, print feed names
	fmt.Printf("%s is following:\n", user.Name)
	for i := range follows {
		fmt.Printf("  - %s\n", follows[i].FeedName)
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
		return err
	}

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

func handlerUnfollow(s *state, cmd command, user database.User) error {
	// Verify 1 argument included for URL
	if len(cmd.args) != 1 {
		return errors.New("URL required for unfollow command")
	}

	// Get Feed by URL
	feed, err := s.db.GetFeedByURL(context.Background(), cmd.args[0])
	if err != nil {
		return err
	}

	// Execute unfollow query
	params := database.DeleteFeedFollowParams{
		UserID: user.ID,
		FeedID: feed.ID,
	}

	if _, err := s.db.DeleteFeedFollow(context.Background(), params); err != nil {
		return err
	}

	fmt.Printf("%s successfully unfollowed %s!\n", user.Name, feed.Name)

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
	for i := range users {
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
	coms.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	coms.register("agg", handlerAgg)
	coms.register("feeds", handlerFeeds)
	coms.register("follow", middlewareLoggedIn(handlerFollow))
	coms.register("following", middlewareLoggedIn(handlerFollowing))
	coms.register("login", handlerLogin)
	coms.register("register", handlerRegister)
	coms.register("reset", handlerReset)
	coms.register("unfollow", middlewareLoggedIn(handlerUnfollow))
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
