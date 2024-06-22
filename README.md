# Go curl exec

Go script to read curl cmds from JSON file and exec them and log the resp times for the curls in json format


### input JSON file format

```json
[
    {
        "name": "name of the curl",
        "command": "curl command using `-o /dev/null -w \"%{time_total}\"`"
        "count": 3
    }
]
```

- It is necessary to use `-o /dev/null -w \"%{time_total}\"` in the curl command to ensure we only get the resp times back and not the response data.

- `count` need not be specified it is optional and its **default value is taken as 3**
