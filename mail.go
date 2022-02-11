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

func setHeader(headers *textproto.MIMEHeader, key, value string) {
	h := textproto.CanonicalMIMEHeaderKey(key)
	headers.Set(h, value)
}

func addHeader(headers *textproto.MIMEHeader, key, value string) {
	h := textproto.CanonicalMIMEHeaderKey(key)
	headers.Add(h, value)
}

// create a multipart MIME message with text and file attachment
func setBody(headers *textproto.MIMEHeader, body *bytes.Buffer, f *excelize.File) error {

	multipartType := "fixed"

	// create a new multipart writer
	writer := multipart.NewWriter(body)
	contentType := "multipart/" + multipartType + ";\n \tboundary=" + writer.Boundary()
	// add header to main header group
	setHeader(headers, "Content-Type", contentType)

	// add text part
	textBody := "\r\nPlease see attached file with portal accounts.\r\n"
	textHeader := make(textproto.MIMEHeader)
	setHeader(&textHeader, "Content-Type", "text/plain; charset="+charSet)
	setHeader(&textHeader, "Content-Transfer-Encoding", bodyEncoding)
	p, err := writer.CreatePart(textHeader)
	if err != nil {
		return err
	}
	_, err = p.Write([]byte(b64.StdEncoding.EncodeToString([]byte(textBody))))
	if err != nil {
		return err
	}

	// add file attachment part
	fp, err := os.Open(f.Path)
	if err != nil {
		return err
	}
	defer fp.Close()

	data, err := io.ReadAll(fp)
	if err != nil {
		return err
	}

	fileHeader := make(textproto.MIMEHeader)
	value := fmt.Sprintf("%s;\n \tname=%q", FileMimeType, AttachedFileName)
	setHeader(&fileHeader, "Content-Type", value)
	setHeader(&fileHeader, "Content-Transfer-Encoding", bodyEncoding)
	value = fmt.Sprintf("attachment;\n \tfilename=%q", AttachedFileName)
	setHeader(&fileHeader, "Content-Disposition", value)
	p, err = writer.CreatePart(fileHeader)
	if err != nil {
		return err
	}
	_, err = p.Write([]byte(b64.StdEncoding.EncodeToString(data)))
	if err != nil {
		return err
	}

	err = writer.Close()
	if err != nil {
		return err
	}

	return nil
}

func toMIMEString(headers *textproto.MIMEHeader, body *bytes.Buffer) string {
	return toHeaderString(headers) + body.String()
}

func toHeaderString(headers *textproto.MIMEHeader) string {
	setHeader(headers, "Date", time.Now().Format(time.RFC1123Z))

	// combine the headers
	strHeader := ""
	for header, values := range *headers {
		strHeader += header + ": " + strings.Join(values, ", ") + "\r\n"
	}

	return strHeader + "\r\n"
}

func validateRecipientAddresses(addresses []string) ([]string, error) {
	// remove duplicate addresses
	// log invalid address and continue
	// if no valid address, set m.err
	validAddresses := make([]string, 0)
	for _, addr := range addresses {
		if len(addr) > 0 {
			a, err := mail.ParseAddress(addr)
			if err != nil {
				log.Printf("validating recipient address %s: %s", addr, err)
				continue
			}
			validAddresses = append(validAddresses, a.Address)
		} else {
			continue
		}
	}
	if len(validAddresses) == 0 {
		return nil, fmt.Errorf("no valid recipient address")
	}

	return validAddresses, nil
}

func sendEmail(f *excelize.File) error {

	host := os.Getenv("MAILSMTPHOST")
	port := os.Getenv("MAILSMTPPORT")

	senderName := os.Getenv("MAILSENDERNAME")
	fromAddress := os.Getenv("MAILFROMADDRESS")
	toAddresses := strings.Split(os.Getenv("MAILTOADDRESSES"), ",")
	headers := make(textproto.MIMEHeader)
	body := new(bytes.Buffer)
	mailEnabled := strings.ToLower(os.Getenv("MAILENABLED"))

	if mailEnabled != "true" {
		log.Println("Mail feature is not enabled")
		return nil
	}

	if senderName == "" || fromAddress == "" {
		return fmt.Errorf("Error sending email: missing sender name and/or sender address")
	}

	validatedToAddresses, err := validateRecipientAddresses(toAddresses)
	if err != nil {
		return fmt.Errorf("Error sending email: %s", err)
	}

	setHeader(&headers, "From", fromAddress)
	for _, address := range validatedToAddresses {
		addHeader(&headers, "To", address)
	}
	setHeader(&headers, "Subject", mailSubject)
	err = setBody(&headers, body, f)
	if err != nil {
		return fmt.Errorf("Error sending email: %s", err)
	}

	msgString := toMIMEString(&headers, body)

	err = smtp.SendMail(host+":"+port, nil, fromAddress, validatedToAddresses, []byte(msgString))
	if err != nil {
		return fmt.Errorf("Error sending mail: %s", err)
	}

	log.Printf("successfully emailed %s", AttachedFileName)

	return nil

}
