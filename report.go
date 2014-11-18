package main

import (
	"crypto/tls"
	"fmt"
	"github.com/goforce/log"
	"net"
	"net/smtp"
)

type EMailConfig struct {
	Server   string   `json:"server"`
	User     string   `json:"user"`
	Password string   `json:"password"`
	From     string   `json:"from"`
	To       []string `json:"to"`
}

type Report struct {
	EMailConfig
	host      string
	successes int
	errors    int
	rows      []string
}

func NewReport(config EMailConfig) *Report {
	var err error
	report := &Report{EMailConfig: config, rows: make([]string, 0, 100)}
	if len(report.To) > 0 {
		report.host, _, err = net.SplitHostPort(report.Server)
		if err != nil {
			panic(fmt.Sprint("failed to parse email server:", err))
		}
	}
	return report
}

func (report *Report) AddSuccess(sObject string, records int) {
	report.successes++
	if len(report.To) > 0 {
		report.rows = append(report.rows, fmt.Sprint(sObject, " - ", records, " records"))
	} else {
		fmt.Println(sObject, " - ", records, " records")
	}
}

func (report *Report) AddError(sObject string, message string) {
	report.errors++
	if len(report.To) > 0 {
		report.rows = append(report.rows, fmt.Sprint(sObject, " - ", message))
	} else {
		fmt.Println(sObject, " - ", message)
	}
}

func (report *Report) Fatal(message string) {
	if len(report.To) > 0 {
		body := "From: salesforce.com backup\n"
		body += "To: you\n"
		body += "Subject: Fatal failure in salesforce.com backup\n\n"
		body += fmt.Sprint("Backup failed to start: ", message, "\n")
		auth := smtp.PlainAuth("", report.User, report.Password, report.host)
		smtp.SendMail(report.Server, auth, report.User, report.To, []byte(body))
	}
	panic(message)
}

func (report *Report) Send() {
	if len(report.To) > 0 {
		body := "From: salesforce.com backup\n"
		body += "To: you\n"
		body += "Subject: salesforce.com backup completed\n\n"
		body += fmt.Sprint("Backup finished with ", report.successes, " objects backed up and ", report.errors, " objects failed.\n")
		body += "Detailed report below:\n"
		for _, r := range report.rows {
			body += r + "\n"
		}
		co, err := tls.Dial("tcp", report.Server, &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         report.host,
		})
		if err != nil {
			log.Panic(fmt.Println("failed to send email: ", err))
		}
		defer co.Close()
		client, err := smtp.NewClient(co, report.host)
		if err != nil {
			log.Panic(fmt.Println("failed to send email: ", err))
		}
		defer client.Quit()
		if err := client.Auth(smtp.PlainAuth("", report.User, report.Password, report.host)); err != nil {
			log.Panic(fmt.Println("failed to send email: ", err))
		}
		if err := client.Mail(report.From); err != nil {
			log.Panic(fmt.Println("failed to send email: ", err))
		}
		for _, to := range report.To {
			if err := client.Rcpt(to); err != nil {
				log.Panic(fmt.Println("failed to send email: ", err))
			}
		}
		writer, err := client.Data()
		if err != nil {
			log.Panic(fmt.Println("failed to send email: ", err))
		}
		defer writer.Close()
		if _, err = writer.Write([]byte(body)); err != nil {
			log.Panic(fmt.Println("failed to send email: ", err))
		}
	} else {
		fmt.Println("Total: ", report.successes, " objects copied, ", report.errors, " object failed")
	}
}
