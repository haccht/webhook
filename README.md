# webhook
A simple webhook server. Run scripts whenever webhook is triggered.

## Usage

```
Usage:
  webhook [OPTIONS]

Application Options:
  -a, --addr=     Address to listen on (default: :8080)
  -f, --file=     Path to the toml file containing hooks definition
      --pid=      Create PID file at the given path
      --tls       Activate https instead of http
      --tls-key=  Path to the private key pem file for HTTPS
      --tls-cert= Path to the certificate pem file for HTTPS

Help Options:
  -h, --help      Show this help message
```


Define some hooks you want to serve in `hooks.toml`.

```
[[hooks]]
name = 'sample'
exec = '/path/to/script.sh'
```

Run `webhook` as below:

```sh
$ webhook --file hooks.toml
2022/08/01 21:00:00 Loaded sample hook
2022/08/01 21:00:00 Listening on :8080
```

It will start up webhook service with HTTP endpoint `http://localhost:8080/sample`.
Then you can execute the script using POST request:

```bash
$ cat input.json | curl -X POST --data-binary @- http://localhost:8080/sample
```

Data in POST request will be passed to the script as stdin.
The output result will be displayed.
