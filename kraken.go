package main

import (
	"crypto/tls"
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message"
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
	from := uint32(1)
	to := mbox.Messages
	if mbox.Messages > 30 {
		// We're using unsigned integers here, only subtract if the result is > 0
		from = mbox.Messages - 30
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)
	items := []imap.FetchItem{imap.FetchItem("BODY.PEEK[]")}
	messages := make(chan *imap.Message, 30)
	if err := c.Fetch(seqset, items, messages); err != nil {
		log.Println(err)
		return
	}

	var checkInLinks []string

	for {
		msg, ok := <-messages
		if !ok {
			break
		}

		for _, literal := range msg.Body {
			mr, err := message.Read(literal)
			if err != nil {
				log.Println(err)
				return
			}

			var buf strings.Builder
			_, err2 := io.Copy(&buf, mr.Body)
			if err2 != nil {
				log.Println(err2)
				return
			}

			email_base64 := buf.String()
			start_index := strings.Index(email_base64, "base64") + 6 + 4
			end_index := strings.Index(email_base64, "=\r\n") + 1
			email_base64 = email_base64[start_index:end_index]
			email_base64 = strings.ReplaceAll(email_base64, "\r\n", "")

			email_bytes, err := base64.StdEncoding.DecodeString(email_base64)
			if err != nil {
				log.Println(err)
				return
			}

			email := string(email_bytes[:])

			if strings.Contains(email, "https://ukg.iofficeconnect.com/external/api/reservation/CheckIn?id=") {
				re := regexp.MustCompile(`https:\/\/ukg\.iofficeconnect\.com\/external\/api\/reservation\/CheckIn\?id=\d{6}&hash=[\d\w]{64}`)
				link := re.FindStringSubmatch(email)
				if len(link) >= 1 {
					// log.Println("* LINK: --- " + link[0])
					checkInLinks = append(checkInLinks, link[0])
				}
			}
			break
		}
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
