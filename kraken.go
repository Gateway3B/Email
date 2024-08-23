package main

import (
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"os"
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
		password = "passwortd"
	}

	log.Println("mailserver: ", mailserver)

	for {
		login(mailserver, email, password)
		time.Sleep(55 * time.Minute)
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

	log.Println("Mailboxes:")
	for m := range mailboxes {
		log.Println("* " + m.Name)
	}

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
	log.Println("Flags for INBOX:", mbox.Flags)

	// Get the last 30 messages
	from := uint32(1)
	to := mbox.Messages
	if mbox.Messages > 30 {
		// We're using unsigned integers here, only subtract if the result is > 0
		from = mbox.Messages - 30
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	messages := make(chan *imap.Message, 30)
	done = make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchItem("BODY.PEEK[]"), imap.FetchEnvelope}, messages)
	}()

	var checkInLinks []string

	log.Println("Last 30 messages:")
	for msg := range messages {
		// log.Println("* " + msg.Envelope.Subject)
		if strings.Contains(msg.Envelope.Subject, "Time to check in!") {
			for _, literal := range msg.Body {
				buf := make([]byte, 1)
				line := ""
				for {
					n, err := literal.Read(buf)
					if err == io.EOF {
						break
					}
					line += string(buf[:n])

					if string(buf[:n]) == "\n" {
						if strings.Contains(line, "https://ukg.iofficeconnect.com/external/api/reservation/CheckIn?id=") {
							checkInLinks = append(checkInLinks, strings.TrimSuffix(line, "\r\n"))
							break
						}
						line = ""
					}
				}
			}
		}
	}

	if err := <-done; err != nil {
		log.Println(err)
	}

	for _, link := range checkInLinks {
		log.Println(link)
		res, err := http.Get(link)
		if err != nil {
			log.Println("Error using sign in link: ", err)
		} else {
			log.Println(res)
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
	}

	log.Println("Done!")
}
