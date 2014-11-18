package main

import (
	"encoding/json"
	"fmt"
	"github.com/goforce/api/commons"
	"github.com/goforce/api/soap"
	"github.com/goforce/log"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

type Config struct {
	Url      string      `json:"url"`
	Username string      `json:"username"`
	Password string      `json:"password"`
	Token    string      `json:"token"`
	Path     string      `json:"path"`
	Include  []string    `json:"include"`
	Skip     []string    `json:"skip"`
	Hours    int         `json:"hours"`
	EMail    EMailConfig `json:"email"`
	Log      string      `json:"log"`
}

func main() {

	restricted := map[string]bool{"contentdocumentlink": true, "ideacomment": true, "vote": true}

	config := ReadConfigFile(os.Args[1])
	log.On(config.Log)

	report := NewReport(config.EMail)

	connection, err := soap.Login(config.Url, config.Username, config.Password+config.Token)
	if err != nil {
		report.Fatal(err.Error())
	}

	globalDescribe, err := connection.DescribeGlobal()
	if err != nil {
		report.Fatal(err.Error())
	}

	includes := make(map[string]bool)
	for _, s := range config.Include {
		includes[strings.ToLower(s)] = true
	}

	skips := make(map[string]bool)
	for _, s := range config.Skip {
		skips[strings.ToLower(s)] = true
	}

	var since *time.Time
	if config.Hours > 0 {
		t := time.Now().Add(-time.Duration(config.Hours) * time.Hour)
		since = &t
	}

	for _, sObject := range globalDescribe.SObjects {
		lcname := strings.ToLower(sObject.Name)
		if sObject.Queryable && sObject.Createable && (len(includes) == 0 || includes[lcname]) && !skips[lcname] && !restricted[lcname] {
			Backup(connection, sObject.Name, report, config.Path, since)
		}
	}

	report.Send()

}

func ReadConfigFile(filename string) *Config {
	configInput, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(fmt.Sprint("cannot open config file: ", filename, "\n", err))
	}
	config := &Config{}
	err = json.Unmarshal(configInput, config)
	if err != nil {
		panic(fmt.Sprint("error parsing config file: ", filename, "\n", err))
	}
	return config
}

func Backup(connection *soap.Connection, sObject string, report *Report, path string, since *time.Time) {

	defer func() {
		if r := recover(); r != nil {
			report.AddError(sObject, fmt.Sprint(r))
		}
	}()

	// compile soql statement
	describe, err := connection.DescribeSObject(sObject)
	if err != nil {
		panic(err)
	}
	fields := make([]string, 0, len(describe.Fields))
	hasModstamp := false
	hasLastModifiedDate := false
	hasCreatedDate := false
	for _, fd := range describe.Fields {
		if fd.Type != "address" && fd.Type != "location" {
			//			if fd.Type == "base64" {
			//				fmt.Println("------ base64 field --------- : ", describe.Name, " -> ", fd.Name)
			//			}
			fields = append(fields, fd.Name)
			if len(fd.ReferenceTo) == 1 && fd.ReferenceTo[0] != "User" && len(fd.RelationshipName) > 0 {
				cd, err := connection.DescribeSObject(fd.ReferenceTo[0])
				if err != nil {
					panic(err)
				}
				for _, cfd := range cd.Fields {
					if cfd.IdLookup || cfd.NamePointing {
						fields = append(fields, fd.RelationshipName+"."+cfd.Name)
					}
				}
			}
			if fd.Name == "SystemModstamp" {
				hasModstamp = true
			}
			if fd.Name == "LastModifiedDate" {
				hasLastModifiedDate = true
			}
			if fd.Name == "CreatedDate" {
				hasCreatedDate = true
			}
		}
	}

	writer := NewWriter(path+describe.Name+".csv", fields)

	soql := "select " + strings.Join(fields, ",") + " from " + describe.Name
	if since != nil {
		if hasModstamp {
			soql += " where SystemModstamp >= "
		} else if hasLastModifiedDate {
			soql += " where LastModifiedDate >= "
		} else if hasCreatedDate {
			soql += " where CreatedDate >= "
		} else {
			return
		}
		soql += since.Format("2006-01-02T15:04:05.999Z0700")
	}

	reader, err := commons.NewReader(connection.Query(soql))
	if err != nil {
		panic(err)
	}

	num := 0

	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}
		num++
		writer.Write(rec)
	}
	writer.Close()
	report.AddSuccess(sObject, num)
}
