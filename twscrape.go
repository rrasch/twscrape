package main

import (
	"context"
	"encoding/json"
	"fmt"
	twitterscraper "github.com/n0madic/twitter-scraper"
	toml "github.com/pelletier/go-toml"
	"github.com/syndtr/goleveldb/leveldb"
	"net/http"
	"net/smtp"
	"os"
	"path"
	"strings"
)

func SendTweet(from string, to []string, tweet *twitterscraper.TweetResult) {
	subject := fmt.Sprintf("New Tweet from %s", tweet.Username)
	message := tweet.Text

	smtpServer := "localhost:25"

	body := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", strings.Join(to, ", "), subject, message)

	err := smtp.SendMail(smtpServer, nil, from, to, []byte(body))
	if err != nil {
		panic(err)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <twitter handle>\n\n", os.Args[0])
}

func main() {
	if len(os.Args[1:]) != 1 {
		usage()
		os.Exit(1)
	}

	twitterHandle := os.Args[1]

	dirname, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	tomlPath := path.Join(dirname, "work", "python-scripts", "config.toml")
	config, err := toml.LoadFile(tomlPath)
	if err != nil {
		panic(err)
	}

	mailFrom := config.Get("main.mailfrom").(string)
	mailTo := config.GetArray("main.mailto").([]string)

	// fmt.Println(mailFrom)
	// fmt.Println(mailTo)

	twitterUser := config.Get("twitter.username").(string)
	twitterPass := config.Get("twitter.password").(string)

	dbPath := path.Join(dirname, "logs", "tweet.ldb")
	// fmt.Println(dbPath)

	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	/*
	   iter := db.NewIterator(nil, nil)
	   for iter.Next() {
	       // Remember that the contents of the returned slice should not be modified, and
	       // only valid until the next call to Next.
	       key := iter.Key()
	       value := iter.Value()
	       fmt.Println(string(key))
	       fmt.Println(string(value))
	   }
	   iter.Release()
	*/

	scraper := twitterscraper.New()

	cookiesPath := path.Join(dirname, ".twcookies.json")
	f, _ := os.Open(cookiesPath)
	// deserialize from JSON
	var cookies []*http.Cookie
	json.NewDecoder(f).Decode(&cookies)
	// load cookies
	scraper.SetCookies(cookies)
	if err := f.Close(); err != nil {
		panic(err)
	}

	if !scraper.IsLoggedIn() {
		err := scraper.Login(twitterUser, twitterPass)
		if err != nil {
			panic(err)
		}
	}

	for tweet := range scraper.GetTweets(context.Background(), twitterHandle, 10) {
		if tweet.Error != nil {
			panic(tweet.Error)
		}

		tweetExists, _ := db.Has([]byte(tweet.ID), nil)
		if !tweetExists {
			SendTweet(mailFrom, mailTo, tweet)
			err = db.Put([]byte(tweet.ID), []byte(tweet.Text), nil)
		}
	}

	new_cookies := scraper.GetCookies()
	// serialize to JSON
	js, _ := json.Marshal(new_cookies)
	// save to file
	f, _ = os.Create(cookiesPath)
	f.Write(js)
}
