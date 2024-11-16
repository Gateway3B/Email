package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

func main() {
	mailserver, mailseverexists := os.LookupEnv("mailserver")
	if !mailseverexists {
		mailserver = "front"
	}
	mailserver = mailserver + ":993"

	email, emailexists := os.LookupEnv("email")
	if !emailexists {
		email = "kraken@mail.g3tech.net"
	}

	password, passwordexists := os.LookupEnv("password")
	if !passwordexists {
		password = "password"
	}

	log.Println("mailserver: ", mailserver)

	for {
		login(mailserver, email, password)
		time.Sleep(20 * time.Minute)
	}
}

func login(mailserver string, email string, password string) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	c, err := client.DialTLS(mailserver, tlsConfig)
	if err != nil {
		log.Println("failed to dial IMAP server: ", err)
		return
	}
	log.Println("Connected")
	defer c.Logout()
	defer c.Close()

	if err := c.Login(email, password); err != nil {
		log.Println("failed to login: ", err)
		return
	}
	log.Println("Logged in")

	// List mailboxes
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.List("", "*", mailboxes)
	}()

	// log.Println("Mailboxes:")
	// for m := range mailboxes {
	// 	log.Println("* " + m.Name)
	// }

	if err := <-done; err != nil {
		log.Println(err)
		return
	}

	// Select INBOX
	mbox, err := c.Select("INBOX", false)
	if err != nil {
		log.Println(err)
		return
	}
	// log.Println("Flags for INBOX:", mbox.Flags)

	// Get the last 30 messages
	emailCount := uint32(30)
	from := uint32(1)
	to := mbox.Messages
	if mbox.Messages > emailCount {
		// We're using unsigned integers here, only subtract if the result is > 0
		from = mbox.Messages - emailCount
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	messages := make(chan *imap.Message, emailCount)
	done = make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
	}()

	var checkInLinks []string

	for msg := range messages {
		subject := msg.Envelope.Subject
		if strings.Contains(subject, "https://ukg.iofficeconnect.com/external/api/reservation/CheckIn?id=") {
			re := regexp.MustCompile(`https:\/\/ukg\.iofficeconnect\.com\/external\/api\/reservation\/CheckIn\?id=\d{6}&hash=[\d\w]{64}`)
			link := re.FindStringSubmatch(subject)
			if len(link) >= 1 {
				// log.Println("* LINK: --- " + link[0])
				checkInLinks = append(checkInLinks, link[0])
			}
		}
	}

	if err := <-done; err != nil {
		log.Println(err)
		return
	}

	log.Println("Check In Link Count: " + strconv.Itoa(len(checkInLinks)))
	for _, link := range checkInLinks {
		log.Print(link)
		log.Print(" : ")
		_, err := http.Get(link)
		if err != nil {
			log.Println("Error using sign in link: ", err)
		} else {
			log.Println("Sign in Success")
		}
	}

	criteria := imap.SearchCriteria{}
	ids, err := c.Search(&criteria)
	if err != nil {
		log.Println(err)
		return
	}

	// Mark all messages for deletion
	if len(ids) > 0 {
		// Create a SeqSet with the IDs of the messages to be deleted
		seqset = new(imap.SeqSet)
		seqset.AddNum(ids...)

		if err := c.Store(seqset, "+FLAGS", []interface{}{imap.DeletedFlag}, nil); err != nil {
			log.Println(err)
			return
		}

		// Expunge the mailbox to permanently delete the messages
		if err := c.Expunge(nil); err != nil {
			log.Println(err)
			return
		}

		log.Println("Inbox deleted")
	} else {
		log.Println("No login emails detected")
	}

	log.Println("Done!")
}
