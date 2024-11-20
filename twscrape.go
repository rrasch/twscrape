package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
// 	twitterscraper "github.com/n0madic/twitter-scraper"
	twitterscraper "github.com/imperatrona/twitter-scraper"
	toml "github.com/pelletier/go-toml"
	log "github.com/sirupsen/logrus"
	"github.com/ssgelm/cookiejarparser"
	"github.com/syndtr/goleveldb/leveldb"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"html"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)


func ReadJSONCookies(cookiesPath string) []*http.Cookie {
	f, _ := os.Open(cookiesPath)
	// deserialize from JSON
	var cookies []*http.Cookie
	json.NewDecoder(f).Decode(&cookies)
	if err := f.Close(); err != nil {
		panic(err)
	}
	return cookies
}


func ReadNetscapeCookies(cookiesPath string) []*http.Cookie {
	cookiejar, err := cookiejarparser.LoadCookieJarFile(cookiesPath)
	if err != nil {
		panic(err)
	}
	u, _ := url.Parse("https://x.com")
	cookies := cookiejar.Cookies(u)
	return cookies
}


func PrintCookies(cookies []*http.Cookie) {
	for _, cookie := range cookies {
		log.Debug("Cookie ", cookie.Name, ": ", cookie.Value)
	}
}


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


func init() {
	log.SetFormatter(&prefixed.TextFormatter{
		DisableColors:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
		ForceFormatting: true,
	})
	log.SetOutput(os.Stderr)
}


func main() {
	debug := flag.Bool("debug", false, "enable debugging")
	flag.Parse()

	var level log.Level
	if *debug {
		level = log.DebugLevel
	} else {
		level = log.InfoLevel
	}
	log.SetLevel(level)

	if flag.NArg() != 1 {
		usage()
		os.Exit(1)
	}

// 	logger := &logrus.Logger{
// 		Out:   os.Stderr,
// 		Level: level,
// 		Formatter: &prefixed.TextFormatter{
// 			DisableColors:   true,
// 			TimestampFormat: "2006-01-02 15:04:05",
// 			FullTimestamp:   true,
// 			ForceFormatting: true,
// 		},
// 	}

	twitterHandle := flag.Arg(0)

	homedir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	tomlPath := path.Join(homedir, "work", "python-scripts", "config.toml")
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

	log.Debug("mail server: ", mailServer)
	log.Debug("mail from: ", mailFrom)
	log.Debug("mail to: ", mailTo)

// 	twitterUser := config.Get("twitter.username").(string)
// 	twitterPass := config.Get("twitter.password").(string)

	dbPath := path.Join(homedir, "logs", "tweet.ldb")
	log.Debug("db path: ", dbPath)

	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	iter := db.NewIterator(nil, nil)
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()
		log.Debug("key: ", string(key))
		log.Debug("value: ", string(value))
	}
	iter.Release()
	os.Exit(0)

	jsonCookiesPath := path.Join(homedir, ".twcookies.json")
	netscapeCookiesPath := path.Join(homedir, "Downloads", "x.com_cookies.txt")

	var cookies []*http.Cookie
	if _, err := os.Stat(jsonCookiesPath); err == nil {
		cookies = ReadJSONCookies(jsonCookiesPath)
	} else {
		cookies = ReadNetscapeCookies(netscapeCookiesPath)
	}
	PrintCookies(cookies)

	scraper := twitterscraper.New()
	scraper.SetCookies(cookies)

	if !scraper.IsLoggedIn() {
// 		log.Debug("Logging in as user ", twitterUser)
// 		err := scraper.Login(twitterUser, twitterPass)
// 		if err != nil {
// 			panic(err)
// 		}
		panic("Not Logger In")
	}

	log.Debug("Getting tweets.")

	for tweet := range scraper.GetTweets(context.Background(), twitterHandle, 10) {
		if tweet.Error != nil {
			panic(tweet.Error)
		}

		log.Debugf("tweet: %+v\n", tweet)

		msgText := fmt.Sprintf(
			"%s <%s> %s\n%s",
			tweet.TimeParsed.Local().Format(time.RFC1123),
			tweet.Username,
			tweet.Text,
			tweet.PermanentURL,
		)
		msgText = html.UnescapeString(msgText)
		log.Debug("msg text: ", msgText)

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
	f, _ := os.Create(jsonCookiesPath)
	f.Write(js)
}
