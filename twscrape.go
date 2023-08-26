package main

import (
    "encoding/json"
    "context"
    "fmt"
    "github.com/syndtr/goleveldb/leveldb"
    "net/http"
    "net/smtp"
    "os"
    "path"
    "reflect"
    "strings"
    toml "github.com/pelletier/go-toml"
    twitterscraper "github.com/n0madic/twitter-scraper"
)

func main() {
    twitter_username = os.Args[1]

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

    fmt.Println(mailFrom)
    fmt.Println(mailTo)

    twitterUser = config.Get("twitter.username").(string)
    twitterPass = config.Get("twitter.password").(string)

    dbPath := path.Join(dirname, "logs", "tweet.ldb")
    fmt.Println(dbPath)

    db, err:= leveldb.OpenFile(dbPath, nil)
    if err != nil {
        panic(err)
    }
    defer db.Close()

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

    scraper := twitterscraper.New()

    f, _ := os.Open("cookies.json")
    // deserialize from JSON
    var cookies []*http.Cookie
    json.NewDecoder(f).Decode(&cookies)
    // load cookies
    scraper.SetCookies(cookies)

    if ! scraper.IsLoggedIn() {
        err := scraper.Login(twitterUser, twitterPass)
        if err != nil {
            panic(err)
        }
    }

    for tweet := range scraper.GetTweets(context.Background(), twitter_username, 10) {
        if tweet.Error != nil {
            panic(tweet.Error)
        }
        fmt.Println()
        fmt.Println(reflect.TypeOf(tweet))
        fmt.Println(tweet.ID)
        fmt.Println(tweet.ConversationID)
        fmt.Println(tweet.Text)
    }

    new_cookies := scraper.GetCookies()
    // serialize to JSON
    js, _ := json.Marshal(new_cookies)
    // save to file
    f, _ = os.Create("cookies.json")
    f.Write(js)

    // Set up the email configuration
    subject := "Test Email"
    message := "This is a test email sent from Go."

//     // Connect to the local SMTP server (replace localhost:25 with the appropriate address)
//     auth := smtp.PlainAuth("", "", "", "localhost")
    smtpServer := "localhost:25"

    // Compose the email
    body := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", strings.Join(mailTo, ", "), subject, message)

    // Send the email
    err = smtp.SendMail(smtpServer, nil, mailFrom, mailTo, []byte(body))
    if err != nil {
        panic(err)
    }

    fmt.Println("Email sent successfully")

}

