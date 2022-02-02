package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"strings"
	"time"

	b64 "encoding/base64"

	"github.com/xuri/excelize/v2"
)

const (
	mailSubject      = "MACFin portal test accounts"
	AttachedFileName = "MACFin_test_accounts.xlsx"
	bodyEncoding     = "base64"
	charSet          = "UTF-8"
)

const (
	FileMimeType           = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	FileContentDisposition = "attachment"
)

const (
	SMTPHost = "internal-Enterpris-SMTPProd-I20YLD1GTM6L-357506541.us-east-1.elb.amazonaws.com"
	SMTPPort = "25"
)

type SMTPRelay struct {
	Host string
	Port string
}

type Message struct {
	fromAddress string
	senderName  string
	toAddresses []string
	subject     string
	headers     textproto.MIMEHeader
	body        *bytes.Buffer
	err         error
	mailEnabled string
}

type File struct {
	name     string
	mimeType string
	data     []byte
}

func newFile(f *excelize.File) (*File, error) {
	fp, err := os.Open(f.Path)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	data, err := io.ReadAll(fp)
	if err != nil {
		return nil, err
	}

	return &File{
		name:     AttachedFileName,
		mimeType: FileMimeType,
		data:     data,
	}, nil

}

func (m *Message) setHeader(header, value string) *Message {
	h := textproto.CanonicalMIMEHeaderKey(header)
	m.headers.Set(h, value)

	return m
}

func (m *Message) addHeader(header, value string) *Message {
	h := textproto.CanonicalMIMEHeaderKey(header)
	m.headers.Add(h, value)

	return m
}

func (m *Message) addRecipientAddresses(addresses ...string) *Message {
	for _, addr := range addresses {
		m.addHeader("To", addr)
	}
	return m
}

func (m *Message) SetFrom() *Message {
	if m.err != nil {
		return m
	}

	m.setHeader("From", m.senderName+" <"+m.fromAddress+">")
	return m
}

func (m *Message) SetSubject() *Message {
	if m.err != nil {
		return m
	}

	m.setHeader("Subject", m.subject)
	return m
}

func (m *Message) AddTo(addresses ...string) *Message {
	if m.err != nil {
		return m
	}

	m.addRecipientAddresses(addresses...)

	return m
}

// create a multipart MIME message with text and file attachment
func (m *Message) SetBody(multipartType string, f *excelize.File) *Message {
	if m.err != nil {
		return m
	}

	var header textproto.MIMEHeader

	// create a new multipart writer
	writer := multipart.NewWriter(m.body)
	contentType := "multipart/" + multipartType + ";\n \tboundary=" + writer.Boundary()
	// add header to main header group
	m.setHeader("Content-Type", contentType)

	// add text part
	textBody := "\r\nPlease see attached file with portal accounts.\r\n"
	header = make(textproto.MIMEHeader)
	header.Set("Content-Type", "text/plain; charset="+charSet)
	header.Set("Content-Transfer-Encoding", bodyEncoding)
	p, err := writer.CreatePart(header)
	if err != nil {
		m.err = err
		return m
	}
	_, err = p.Write([]byte(b64.StdEncoding.EncodeToString([]byte(textBody))))
	if err != nil {
		m.err = err
		return m
	}

	// add file attachment part
	ft, err := newFile(f)
	if err != nil {
		m.err = err
		return m
	}

	header = make(textproto.MIMEHeader)
	value := fmt.Sprintf("%s;\n \tname=%q", ft.mimeType, ft.name)
	header.Set("Content-Type", value)
	header.Set("Content-Transfer-Encoding", bodyEncoding)
	value = fmt.Sprintf("attachment;\n \tfilename=%q", ft.name)
	header.Set("Content-Disposition", value)
	p, err = writer.CreatePart(header)
	if err != nil {
		m.err = err
		return m
	}
	_, err = p.Write([]byte(b64.StdEncoding.EncodeToString(ft.data)))
	if err != nil {
		m.err = err
		return m
	}

	err = writer.Close()
	if err != nil {
		m.err = err
		return m
	}

	return m
}

func (m *Message) MIMEString() string {
	return m.getHeaders() + m.body.String()
}

func (m *Message) getHeaders() (headers string) {
	m.setHeader("Date", time.Now().Format(time.RFC1123Z))

	// combine the headers
	for header, values := range m.headers {
		headers += header + ": " + strings.Join(values, ", ") + "\r\n"
	}

	headers = headers + "\r\n"

	return

}

func (m *Message) validateRecipientAddresses() *Message {
	// remove duplicate addresses
	// log invalid address and continue
	// if no valid address, set m.err
	addresses := make([]string, 0)
	addressSet := make(map[string]bool)
	for _, addr := range m.toAddresses {
		if len(addr) > 0 {
			a, err := mail.ParseAddress(addr)
			if err != nil {
				log.Printf("validating recipient address %s: %s", addr, err)
				continue
			}
			if _, ok := addressSet[a.Address]; !ok {
				addressSet[a.Address] = true
				addresses = append(addresses, a.Address)
			}
		} else {
			continue
		}
	}
	if len(addresses) == 0 {
		m.err = fmt.Errorf("no valid recipient address")
		return m
	}

	m.toAddresses = addresses

	return m
}

func sendEmail(f *excelize.File) error {
	smtpRelay := &SMTPRelay{
		Host: os.Getenv("MAILSMTPHOST"),
		Port: os.Getenv("MAILSMTPPORT"),
	}
	msg := &Message{
		senderName:  os.Getenv("MAILSENDERNAME"),
		fromAddress: os.Getenv("MAILFROMADDRESS"),
		toAddresses: strings.Split(os.Getenv("MAILTOADDRESSES"), ","),
		subject:     mailSubject,
		headers:     make(textproto.MIMEHeader),
		body:        new(bytes.Buffer),
		mailEnabled: strings.ToLower(os.Getenv("MAILENABLED")),
	}

	if msg.mailEnabled != "true" {
		log.Println("Mail feature is not enabled")
		return nil
	}

	// check msg fields
	if msg.senderName == "" || msg.fromAddress == "" {
		return fmt.Errorf("Error sending email: missing sender name and/or sender address")
	}

	msg.validateRecipientAddresses()

	msg.SetFrom().AddTo(msg.toAddresses...).SetSubject().SetBody("mixed", f)
	if msg.err != nil {
		return fmt.Errorf("Error sending email: %s", msg.err)
	}

	msgString := msg.MIMEString()

	err := smtp.SendMail(smtpRelay.Host+":"+smtpRelay.Port, nil, msg.fromAddress, msg.toAddresses, []byte(msgString))
	if err != nil {
		return fmt.Errorf("Error sending mail: %s", err)
	}

	log.Printf("successfully emailed %s", AttachedFileName)

	return nil

}
