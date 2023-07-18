# SOCKS 4

SOCKS 4 implement a proxy server support SOCKS 4 and 4A protocol.

Supports:
- SOCKS 4 connect command;
- SOCKS 4 bind command;
- Capable of resolving domain names (SOCKS 4A);

## Install

```
$ git clone https://github.com/cccxg/socks4.git
```

## Usage

Start the SOCKS 4 proxy server.
```
$ go run cmd/main.go
time="2023-07-18 23:00:30" level=info msg="SOCKS server listen on :1080"
......
```

## Contributing

PRs accepted.

## License

MIT © Richard McRichface