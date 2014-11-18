back-up
=======

Make copy of your salesforce.com data to local csv files

Usage: back-up config-file.json

Sample config file below:

```Json
{
	"url": "https://login.salesforce.com",
	"username": "salesforce username",
	"password": "salesforce user password",
	"token": "salesforce security token",
	"path": "path to backup files/",
	"hours": 48,
	"include": [],
	"skip": [],
	"email": {
		"server": "smtp server and port",
		"user": "user",
		"password": "password",
		"to": [ "list of email to send notifications to" ]
	},
    "log": "api.calls api.messages api.errors api.durations"
}
```

`hours` - 0 - full backup, any other number - hours since last change to be backup. (optional);

`include` - SObjects to include in backup. Leave empty or remove to backup all. (optional);

`skip` - SObjects to be skipped from backup. Leave emtpy or remove to backup only included or all. (optional);

`email` - remove if notification email not needed. (optional);

`log` - add if tired to look at emtpy window. (optional),