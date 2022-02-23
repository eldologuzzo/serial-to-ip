// serial-to-ip
// File:    serial-to-ip.go
// Author:  annlumia, Eldo Loguzzo
// Date:    2022/02/23
// Version: 1.0.2
//
//
// TODO
//==============================================================================================
// Date              Author        Note
//==============================================================================================
//==============================================================================================
//
// CHANTELOG
//==============================================================================================
// Date              Author    Version   Description
//==============================================================================================
// 2022022309024EX   Eldo      1.0.2     Se permiten modificarlos buffer para Lectura de ambos
//                                       protocolos
// 202202230900MEX   Eldo      1.0.1     Se Permite modificar el Timer que tiene en el loop de
//                                       recepcion. se agrega un timer luego de cada escritura
// 202202170900MEX   Eldo      0.9.0     Se agrega la capa de logueo y mas parametros de
//                                       configuracion
// 202202150720MEX   Eldo      0.1.0     Initital - Deriva del original de annlumia
//                                       https://github.com/annlumia/serial2ip
//==============================================================================================

package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strconv"
	"time"

	"flag"

	log "github.com/jeanphorn/log4go"

	"encoding/hex"

	"github.com/eldologuzzo/serial"
)

var Version = "1.0.2"

var (
	help                 = flag.Bool("help", false, "Help")
	Logger               = flag.String("Logger", "logger.properties", "Logger Config File")
	serialPortName       = flag.String("serial-port", "COM2", "Port name of serial")
	serialPortParity     = flag.String("parity", "E", "Serial port parity (N, O, E,M, S)")
	serialPortStopBit    = flag.Int("stop-bits", 1, "Serial port stop bits (1, 15, 2)")
	baudrate             = flag.Int("baudrate", 9600, "Baudrate of serial port")
	output               = flag.Int("tcp-port", 9000, "TCP port output")
	responseInterval     = flag.String("response-interval", "1000ms", "delay before reading loop")
	serialWriteDelay     = flag.String("serial-write-delay", "100ms", "delay after read serial response")
	serialPortBufferSize = flag.Int("serial-buffer-size", 64, "Serial Port Buffer Size")
	tcpPortBufferSize    = flag.Int("tcp-buffer-size", 64, "TCP Buffer Size")
)

var Response_Interval time.Duration

func main() {
	var SleepTime time.Duration

	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "%s v%s (%s/%s/%s)\n", os.Args[0], Version, runtime.GOOS, runtime.GOARCH, runtime.Version())
		_, _ = fmt.Fprintf(os.Stderr, "\nSyntax and Help\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "Options: \n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if !flag.Parsed() || *help {
		flag.Usage()
		os.Exit(1)
	}

	log.LoadConfiguration(*Logger)
	defer log.Close()

	log.Info("Serial Port to IP converter (%s) v%s (%s/%s/%s)", os.Args[0], Version, runtime.GOOS, runtime.GOARCH, runtime.Version())
	log.Info("Config:")
	log.Info("   serial-port=%s", *serialPortName)
	log.Info("   baudrate=%d", *baudrate)
	log.Info("   parity=%s", *serialPortParity)
	log.Info("   stop-bits=%d", *serialPortStopBit)
	log.Info("   tcp-port=%d", *output)
	log.Info("   Logger=%s", *Logger)
	log.Info("   response-interval=%s", *responseInterval)
	log.Info("   serial-write-delay=%s", *serialWriteDelay)
	log.Info("   serial-buffer-size=%d", *serialPortBufferSize)
	log.Info("   tcp-buffer-size=%d", *tcpPortBufferSize)

	parity := serial.ParityEven
	switch *serialPortParity {
	case "N":
		parity = serial.ParityNone
	case "E":
		parity = serial.ParityEven
	case "O":
		parity = serial.ParityOdd
	case "M":
		parity = serial.ParityMark
	case "S":
		parity = serial.ParitySpace
	}

	serConfig := serial.Config{Name: *serialPortName, Baud: *baudrate, Parity: parity, StopBits: serial.StopBits(*serialPortStopBit)}

	serPort, err := serial.OpenPort(&serConfig)

	if err != nil {
		log.Error("Can not open serial port: %s", err)
		os.Exit(3)
	}

	SleepTime, Error := time.ParseDuration(*serialWriteDelay)

	if Error != nil {
		_ = log.Error("The erial-write-delay Parameter format invalid %s: %v", *serialWriteDelay, Error)
		os.Exit(4)
	}

	Response_Interval, Error := time.ParseDuration(*responseInterval)

	if Error != nil {
		_ = log.Error("The responseInterval Parameter format invalid %s: %v", *responseInterval, Error)
		os.Exit(4)
	}

	// Forzado, ya que sino no detecta el Uso de Reponse_invertal
	<-time.After(Response_Interval)

	defer serPort.Close()

	listener, err := net.Listen("tcp", ":"+strconv.Itoa(*output))
	defer listener.Close()

	ser2ipBuf := make([]byte, *serialPortBufferSize)
	ip2serBuf := make([]byte, *tcpPortBufferSize)

	serPortReadChan := make(chan readResult)
	serPortReadMore := make(chan bool)
	go readProc(serPort, ser2ipBuf, serPortReadChan, serPortReadMore)

	ipReadChan := make(chan readResult)

	acceptChan := make(chan acceptResult)
	acceptMore := make(chan bool)
	go acceptProc(listener, acceptChan, acceptMore)

	// Things that belong to the current connection
	var currentCon net.Conn = nil
	var currentReadMore chan bool = nil
	var connErr error = nil
	var serialErr error = nil

	log.Debug("main - go to MainLoop")

	for {
		select {
		case readResult := <-serPortReadChan:

			log.Debug("main - Read From Serial: %d", readResult.bytesRead)

			encodedString := hex.EncodeToString(ser2ipBuf[0:readResult.bytesRead])

			log.Trace("main - [%s]", encodedString)

			if readResult.err != nil {
				serialErr = readResult.err
			} else {
				if currentCon != nil {
					_, connErr = currentCon.Write(ser2ipBuf[0:readResult.bytesRead])
					log.Debug("main - Write to IP: %d", readResult.bytesRead)
				}

				if connErr != nil {
					log.Error("main - Write IP")
				}

				serPortReadMore <- true

				log.Debug("main - End Serial->IP")

			}
		case readResult := <-ipReadChan:
			log.Debug("main - Read From IP: %d", readResult.bytesRead)

			encodedString := hex.EncodeToString(ip2serBuf[0:readResult.bytesRead])

			log.Trace("main - [%s]", encodedString)

			if readResult.err != nil {
				// Error reading from ip connection
				connErr = readResult.err
			} else {
				_, serialErr = serPort.Write(ip2serBuf[0:readResult.bytesRead])

				log.Debug("main - Write to Serial: %d", readResult.bytesRead)

				if serialErr == nil {
					// Read more
					currentReadMore <- true
				} else {
					log.Error("main - Write Serial")

				}

				time.Sleep(SleepTime)
				log.Debug("main - End IP->Serial")

			}
		case acceptResult := <-acceptChan:
			log.Debug("main - Accept IP Connection")

			if acceptResult.err != nil {
				log.Error("main - Can not accept connection: %s", acceptResult.err)
				return
			} else {
				currentCon = acceptResult.conn
				currentReadMore = make(chan bool)
				go readProc(currentCon, ip2serBuf, ipReadChan, currentReadMore)
			}
		}

		if serialErr != nil {
			log.Error("main - Error reading from serial port: %s", serialErr)
			if currentCon != nil {
				currentCon.Close()
				return
			}
		}

		if currentCon != nil && connErr != nil {
			log.Debug("main - Close Connection")
			// Close the connection and accept a new one
			currentCon.Close()
			currentCon = nil
			connErr = nil
			acceptMore <- true
		}
	}

}

type readResult struct {
	bytesRead int
	err       error
}

type acceptResult struct {
	conn net.Conn
	err  error
}

// Reads from a reader and returns the results in a channel
// After that reading will be stopped until readMore is signaled to give the
// receiver a chance to work with everything in the buffer before we overwrite it
func readProc(src io.Reader, buf []byte, result chan readResult, readMore chan bool) {

	log.Trace("readProc - Entry")

	for {

		<-time.After(Response_Interval)
		n, err := src.Read(buf)
		log.Trace("readProc - readResult go")
		result <- readResult{bytesRead: n, err: err}
		log.Trace("readProc - return readResult")

		_, ok := <-readMore
		if !ok {
			log.Trace("readProc - Exit !ok")
			return
		}
	}
}

// Accepts connections in the goroutine
// After accepting a single connection accepting will be stopped until acceptMore is signaled
func acceptProc(listener net.Listener, result chan acceptResult, acceptMore chan bool) {
	log.Trace("acceptProc - Entry")

	for {
		conn, err := listener.Accept()
		log.Trace("acceptProc - acceptResult go")
		result <- acceptResult{conn: conn, err: err}

		log.Trace("acceptProc - return acceptResult")

		_, ok := <-acceptMore
		if !ok {
			log.Trace("acceptProc - Exit !ok")

			return
		}
	}
}
