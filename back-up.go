package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	Exclude  []string    `json:"exclude"`
	Hours    int         `json:"hours"`
	EMail    EMailConfig `json:"email"`
	Log      string      `json:"log"`
}

type Context struct {
	Co        *soap.LoginResponse
	Describes map[string]*soap.DescribeSObjectResult
	Report    *Report
	Path      string
}

func main() {

	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
		}
	}()

	if len(os.Args) < 2 || (os.Args[1] == "--config" && len(os.Args) < 3) {
		fmt.Println("Usage:")
		fmt.Println("  back-up --config <filename>      - generates sample config file")
		fmt.Println("  back-up <configfilename>         - runs back-up using config file")
		return
	}

	if os.Args[1] == "--config" {
		file, err := os.OpenFile(os.Args[2], os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
		if err != nil {
			fmt.Println("failed to create new config file:", err)
			return
		} else {
			defer file.Close()
			file.WriteString(
				`{
"url": "https://login.salesforce.com",
"username": "yoursalesforce@user.name",
"password": "qwerty",
"token": "1234567890",
"path": "/path/to/backup/files-{YYYY}-{MM}-{DD}",
"include": [ "objects", "to", "include" ],
"comment-include-exclude": "use either include or exclude, empty or missing include means all objects.",
"hours": 24,
"comment-hours": "set 0 or delete for initial backups, for deltas set to frequency.",
"email": {
  "server": "smtp.server:port",
  "user": "user.if.needed",
  "password": "password.if.needed",
  "from": "email.from@me.me",
  "to": [ "first.admin@me.me", "second.admin@me.me" ]
},
"log":"api.soap.duration",
"comment-log": "api.soap.request api.soap.response"
}`,
			)
			return
		}
	}

	restricted := map[string]bool{
		"contentdocumentlink":      true,
		"ideacomment":              true,
		"vote":                     true,
		"support_document__kav":    true,
		"collaborationgrouprecord": true,
	}

	config := ReadConfigFile(os.Args[1])
	log.On(config.Log)

	report := NewReport(config.EMail)

	connection, err := soap.Login(config.Url, config.Username, config.Password+config.Token)
	if err != nil {
		report.Fatal(err.Error())
	}

	globalDescribe, err := soap.DescribeGlobal(connection)
	if err != nil {
		report.Fatal(err.Error())
	}

	includes := make(map[string]bool)
	for _, s := range config.Include {
		includes[strings.ToLower(s)] = true
	}

	excludes := make(map[string]bool)
	for _, s := range config.Exclude {
		excludes[strings.ToLower(s)] = true
	}

	var since *time.Time
	if config.Hours > 0 {
		t := time.Now().Add(-time.Duration(config.Hours) * time.Hour)
		since = &t
	}

	describes := make(map[string]*soap.DescribeSObjectResult)
	names := make([]string, 0, 100)
	for _, sObject := range globalDescribe.SObjects {
		if len(names) >= 100 {
			AddDescribes(connection, describes, names)
			names = names[0:0]
		}
		names = append(names, sObject.Name)
	}
	if len(names) > 0 {
		AddDescribes(connection, describes, names)
	}

	err = os.MkdirAll(config.Path, 0777)
	if err != nil {
		panic(err)
	}

	context := Context{Co: connection, Describes: describes, Report: report, Path: config.Path}

	for _, sObject := range globalDescribe.SObjects {
		lcname := strings.ToLower(sObject.Name)
		if sObject.Queryable && sObject.Createable && (len(includes) == 0 || includes[lcname]) && !excludes[lcname] && !restricted[lcname] {
			Backup(context, sObject.Name, since)
		}
	}

	report.Send()

}

func AddDescribes(connection *soap.LoginResponse, describes map[string]*soap.DescribeSObjectResult, names []string) {
	ds, err := soap.DescribeSObjects(connection, names)
	if err != nil {
		panic("failed to describe sobjects: " + err.Error())
	}
	for _, d := range ds {
		describes[strings.ToLower(d.Name)] = d
	}
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
	if !strings.HasSuffix(config.Path, string(os.PathSeparator)) {
		config.Path = config.Path + string(os.PathSeparator)
	}
	config.Path = strings.Replace(config.Path, "{YYYY}", time.Now().Format("2006"), -1)
	config.Path = strings.Replace(config.Path, "{MM}", time.Now().Format("01"), -1)
	config.Path = strings.Replace(config.Path, "{DD}", time.Now().Format("02"), -1)
	return config
}

func Backup(context Context, sObject string, since *time.Time) {

	defer func() {
		if r := recover(); r != nil {
			context.Report.AddError(sObject, fmt.Sprint(r))
		}
	}()

	// compile soql statement
	describe, ok := context.Describes[strings.ToLower(sObject)]
	if !ok {
		panic("object not described: " + sObject)
	}
	fields := make([]string, 0, len(describe.Fields))
	base64fields := make([]string, 0, 1)
	hasModstamp := false
	hasLastModifiedDate := false
	hasCreatedDate := false
	for _, fd := range describe.Fields {
		if fd.Type != "address" && fd.Type != "location" {
			if fd.Type == "base64" {
				base64fields = append(base64fields, fd.Name)
			} else {
				fields = append(fields, fd.Name)
				if len(fd.ReferenceTo) == 1 && fd.ReferenceTo[0] != "User" && len(fd.RelationshipName) > 0 {
					cd, ok := context.Describes[strings.ToLower(fd.ReferenceTo[0])]
					if !ok {
						fmt.Println("referenced object not described: " + fd.ReferenceTo[0])
					}
					for _, cfd := range cd.Fields {
						if cfd.IdLookup || cfd.NamePointing {
							fields = append(fields, fd.RelationshipName+"."+cfd.Name)
						}
					}
				} else if fd.Name == "SystemModstamp" {
					hasModstamp = true
				} else if fd.Name == "LastModifiedDate" {
					hasLastModifiedDate = true
				} else if fd.Name == "CreatedDate" {
					hasCreatedDate = true
				}
			}
		}
	}

	for _, f := range base64fields {
		err := os.MkdirAll(context.Path+describe.Name+"."+f, 0777)
		if err != nil {
			panic(err)
		}
	}

	writer := NewWriter(context.Path+describe.Name+".csv", fields)

	soql := "select " + strings.Join(append(fields, base64fields...), ",") + " from " + describe.Name
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

	reader, err := soap.NewReader(context.Co, soql)
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
		for _, f := range base64fields {
			if id, ok := rec.Get("Id"); ok {
				if s := rec.MustGet(f); s != nil {
					b, err := base64.StdEncoding.DecodeString(s.(string))
					if err != nil {
						panic(fmt.Sprint("error decoding base64 field ", describe.Name, ".", f, " error:", err.Error()))
					}
					err = ioutil.WriteFile(context.Path+describe.Name+"."+f+string(os.PathSeparator)+id.(string), b, 0666)
					if err != nil {
						panic(fmt.Sprint("failed to write base64 field:", describe.Name, ".", f, " error:", err))
					}
				}
			}
		}
	}
	writer.Close()
	context.Report.AddSuccess(sObject, num)
}
