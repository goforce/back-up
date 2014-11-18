package main

import (
	"fmt"
	"net"
	"net/smtp"
)

type EMailConfig struct {
	Server   string   `json:"server"`
	User     string   `json:"user"`
	Password string   `json:"password"`
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
		smtp.SendMail(report.Server, auth, "backup@mysalesforce.com", report.To, []byte(body))
	}
	panic(message)
}

func (report *Report) Send() {
	if len(report.To) > 0 {
		body := "From: salesforce.com backup\n"
		body += "To: you\n"
		body += "Subject: Fatal failure in salesforce.com backup\n\n"
		body += fmt.Sprint("Backup finished with ", report.successes, " objects backed up and ", report.errors, " objects failed.\n")
		body += "Detailed report below:\n"
		for _, r := range report.rows {
			body += r + "\n"
		}
		auth := smtp.PlainAuth("", report.User, report.Password, report.host)
		err := smtp.SendMail(report.Server, auth, "backup@mysalesforce.com", report.To, []byte(body))
		if err != nil {
			panic(fmt.Sprint("failed to send email:", err))
		}
	} else {
		fmt.Println("Total: ", report.successes, " objects copied, ", report.errors, " object failed")
	}
}
