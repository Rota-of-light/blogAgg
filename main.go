package main

import (
	"fmt"
	"log"
	"github.com/Rota-of-light/blogAgg/internal/config"
	"github.com/Rota-of-light/blogAgg/internal/database"
	"os"
	"database/sql"
	"time"
	"context"
	"github.com/google/uuid"
	"net/http"
	"encoding/xml"
	"io"
	"html"
)

import _ "github.com/lib/pq"

type state struct {
	db		*database.Queries
	config  *config.Config
}

type command struct {
	name string
	args []string
}

type commands struct {
	handlers map[string]func(*state, command) error
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.handlers[name] = f
}

func (c *commands) run(s *state, cmd command) error {
	handler, exists := c.handlers[cmd.name]
	if !exists {
		return fmt.Errorf("Unknown command: %s", cmd.name)
	}
	return handler(s, cmd)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gator")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var feed RSSFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}
	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)
	for i, _ := range feed.Channel.Item {
		feed.Channel.Item[i].Title = html.UnescapeString(feed.Channel.Item[i].Title)
		feed.Channel.Item[i].Description = html.UnescapeString(feed.Channel.Item[i].Description)
	}
	return &feed, nil
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		user, err := s.db.GetUser(context.Background(), s.config.CurrentUserName)
		if err != nil {
			return err
		}
		return handler(s, cmd, user)
	}
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("Username is required.")
	}
	username := cmd.args[0]
	_, err := s.db.GetUser(context.Background(), username)
	if err != nil {
		return fmt.Errorf("User '%s' does not exist", username)
	}
	err = s.config.SetUser(username)
	if err != nil {
		return err
	}
	fmt.Printf("User set to %s\n", username)
	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("Username is required.")
	}
	username := cmd.args[0]
	_, err := s.db.GetUser(context.Background(), username)
	if err == nil {
		return fmt.Errorf("User '%s' already exists", username)
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("Error checking for existing user: %w", err)
	}
	params := database.CreateUserParams{
        ID:        uuid.New(),
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
        Name:      username,
    }
	user, err := s.db.CreateUser(context.Background(), params)
	if err != nil {
		return fmt.Errorf("Error creating user: %w", err)
	}
	err = s.config.SetUser(username)
	if err != nil {
		return err
	}
	fmt.Printf("User '%s' registered successfully\n", username)
	log.Printf("User created: %+v\n", user)
	return nil
}

func handlerReset(s *state, cmd command) error {
	if len(cmd.args) != 0 {
		return fmt.Errorf("No other statements needed when resetting.")
	}
	err := s.db.Reset(context.Background())
	if err != nil {
		return fmt.Errorf("Ran into an error while attempting to reset: %v", err)
	}
	fmt.Println("Reset completed")
	return nil
}

func handlerUsers(s *state, cmd command) error {
	if len(cmd.args) != 0 {
		return fmt.Errorf("No other statements needed when getting usernames.")
	}
	usernames, err := s.db.GetUsernames(context.Background())
	if err != nil {
		return fmt.Errorf("Error when retriving usernames: %v", err)
	} else if len(usernames) == 0 {
		return nil
	}
	current := s.config.CurrentUserName
	for _, name := range usernames {
		if name == current {
			fmt.Printf("* %v (current)\n", name)
		} else {
			fmt.Printf("* %v\n", name)
		}
	}
	return nil
}

func handlerAgg(s *state, cmd command) error {
	if len(cmd.args) != 0 {
		return fmt.Errorf("No other statements needed when using the aggregator, for now.")
	}
	feed, err := fetchFeed(context.Background(), "https://www.wagslane.dev/index.xml")
	if err != nil {
		return fmt.Errorf("Error when getting feed, error: %v", err)
	}
	fmt.Printf("%+v\n", feed)
	return nil
}

func handlerAddFeed(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 2 {
		return fmt.Errorf("Error, either too many arguments or not enough arguments.")
	}
	params := database.CreateFeedParams{
        ID:        uuid.New(),
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
        Name:      cmd.args[0],
		Url:	   cmd.args[1],
		UserID:	   user.ID,
    }
	feed, err := s.db.CreateFeed(context.Background(), params)
	if err != nil {
		return fmt.Errorf("Error creating feed table: %w", err)
	}
	params2 := database.CreateFeedFollowParams{
        ID:        uuid.New(),
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
		UserID:	   user.ID,
		FeedID:	   feed.ID,
    }
	_, err = s.db.CreateFeedFollow(context.Background(), params2)
	if err != nil {
		return fmt.Errorf("Error following feed: %w", err)
	}
	fmt.Printf("%+v\n", feed)
	return nil
}

func handlerFeeds(s *state, cmd command) error {
	if len(cmd.args) != 0 {
		return fmt.Errorf("No other statements needed.")
	}
	feeds, err := s.db.GetFeeds(context.Background())
	if err != nil {
		return fmt.Errorf("Error getting all feeds from table: %w", err)
	}
	for _, feed := range feeds {
		user, err := s.db.GetUserByID(context.Background(), feed.UserID)
		if err != nil {
			return fmt.Errorf("Error getting username: %w", err)
		}
		fmt.Printf("%v | %v | %v\n", feed.Name, feed.Url, user.Name)
	}
	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 1 {
		return fmt.Errorf("Either a feed statement is needed or too many were given.")
	}
	feed, err := s.db.GetFeedsByURLS(context.Background(), cmd.args[0])
	if err != nil {
		return fmt.Errorf("Error getting feed via URL from table: %w", err)
	}
	params := database.CreateFeedFollowParams{
        ID:        uuid.New(),
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
		UserID:	   user.ID,
		FeedID:	   feed.ID,
    }
	result, err := s.db.CreateFeedFollow(context.Background(), params)
	if err != nil {
		return fmt.Errorf("Error following feed: %w", err)
	}
	fmt.Printf("%v | %v\n", result.FeedName,result.UserName)
	return nil
}

func handlerFollowing(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 0 {
		return fmt.Errorf("No other statements needed when getting followed feeds.")
	}
	feeds_followed, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return fmt.Errorf("Error with getting feeds that were followed: %w", err)
	}
	fmt.Printf("%v is following these feeds:\n", user.Name)
	for _, feed := range feeds_followed {
		fmt.Printf("%v\n", feed.FeedName)
	}
	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 1 {
		return fmt.Errorf("Either a url statement is needed or too many were given.")
	}
	feed, err := s.db.GetFeedsByURLS(context.Background(), cmd.args[0])
	if err != nil {
		return fmt.Errorf("Error getting feed via URL from table: %w", err)
	}
	params := database.DeleteFeedFollowParams{
		UserID:	   user.ID,
		FeedID:	   feed.ID,
    }
	err = s.db.DeleteFeedFollow(context.Background(), params)
	if err != nil {
		return fmt.Errorf("Error unfollowing feed: %w", err)
	}
	return nil
}

func main() {
	configInfo, err := config.Read()
	if err != nil {
		log.Fatalf("Issue with reading config, error: %v", err)
	}
	db, err := sql.Open("postgres", configInfo.DBURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	dbQueries := database.New(db)
	s := &state{
		db:		dbQueries,
		config: &configInfo,
	}
	cmds := &commands{
		handlers: make(map[string]func(*state, command) error),
	}
	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	if len(os.Args) <= 1 {
		log.Fatal("Commands and arguments are required")
	}
	cmd := command{
		name: os.Args[1],
		args: os.Args[2:],
	}
	err = cmds.run(s, cmd)
	if err != nil {
		log.Fatalf("%v\n", err)
	}
}
