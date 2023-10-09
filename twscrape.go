package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	twitterscraper "github.com/n0madic/twitter-scraper"
	toml "github.com/pelletier/go-toml"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"html"
	"net/http"
	"net/smtp"
	"os"
	"path"
	"strings"
	"time"
)

func SendTweet(
	server string,
	from string,
	to []string,
	message string,
	tweet *twitterscraper.TweetResult,
) {
	subject := fmt.Sprintf("New Tweet from %s", tweet.Username)

	body := fmt.Sprintf(
		"To: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=\"UTF-8\"\r\n\r\n%s",
		strings.Join(to, ", "),
		subject,
		message,
	)

	err := smtp.SendMail(server, nil, from, to, []byte(body))
	if err != nil {
		panic(err)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <twitter handle>\n\n", os.Args[0])
}

func main() {
	debug := flag.Bool("debug", false, "enable debugging")
	flag.Parse()

	var level logrus.Level
	if *debug {
		level = logrus.DebugLevel
	} else {
		level = logrus.InfoLevel
	}

	if flag.NArg() != 1 {
		usage()
		os.Exit(1)
	}

	logger := &logrus.Logger{
		Out:   os.Stderr,
		Level: level,
		Formatter: &prefixed.TextFormatter{
			DisableColors:   true,
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
			ForceFormatting: true,
		},
	}

	twitterHandle := flag.Arg(0)

	dirname, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	tomlPath := path.Join(dirname, "work", "python-scripts", "config.toml")
	config, err := toml.LoadFile(tomlPath)
	if err != nil {
		panic(err)
	}

	mailHost := config.Get("main.mail_server")
	var mailServer string
	if mailHost == nil {
		mailServer = "localhost"
	} else {
		mailServer = mailHost.(string)
	}
	mailServer += ":25"

	mailFrom := config.Get("main.mailfrom").(string)
	mailTo := config.GetArray("main.mailto").([]string)

	logger.Debug("mail server: ", mailServer)
	logger.Debug("mail from: ", mailFrom)
	logger.Debug("mail to: ", mailTo)

	twitterUser := config.Get("twitter.username").(string)
	twitterPass := config.Get("twitter.password").(string)

	dbPath := path.Join(dirname, "logs", "tweet.ldb")
	logger.Debug("db path: ", dbPath)

	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	iter := db.NewIterator(nil, nil)
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()
		logger.Debug("key: ", string(key))
		logger.Debug("value: ", string(value))
	}
	iter.Release()

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

		logger.Debugf("tweet: %+v\n", tweet)

		msgText := fmt.Sprintf(
			"%s <%s> %s\n%s",
			tweet.TimeParsed.Local().Format(time.RFC1123),
			tweet.Username,
			tweet.Text,
			tweet.PermanentURL,
		)
		msgText = html.UnescapeString(msgText)
		logger.Debug("msg text: ", msgText)

		tweetExists, _ := db.Has([]byte(tweet.ID), nil)
		if !tweetExists {
			SendTweet(mailServer, mailFrom, mailTo, msgText, tweet)
			err = db.Put([]byte(tweet.ID), []byte(msgText), nil)
		}
	}

	new_cookies := scraper.GetCookies()
	// serialize to JSON
	js, _ := json.Marshal(new_cookies)
	// save to file
	f, _ = os.Create(cookiesPath)
	f.Write(js)
}
