# serial2ip

Serial to IP gateway

## System Requirements

- Windows 7 (x86/x64) or newer
- Serial port device

## Connection Ilustration

![Ngantuk yo turu!](./ilustration.PNG 'Ngantuk yo turu')

## How to Use?

Show help

```
$ serial2tcp_x64 -h
```

Sample application

```
$ serial2tcp_x64 -serial-port COM3 -baudrate 19200 -parity E -stop-bits 1 -tcp-port 502
```

## Downloads

For downloading compiled application, please visit [release](https://github.com/annlumia/serial2ip/releases) page.

## TODO

- [x] Serial to TCP/IP gateway
- [ ] Serial to UDP/IP gateway
