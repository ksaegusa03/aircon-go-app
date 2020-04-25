package main

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/line/line-bot-sdk-go/linebot"
)

func main() {
	bot, err := linebot.New(
		os.Getenv("CHANNEL_SECRET"),
		os.Getenv("CHANNEL_TOKEN"),
	)
	if err != nil {
		log.Fatal(err)
	}

	uri, err := url.Parse(os.Getenv("CLOUDMQTT_URL"))
	if err != nil {
		log.Fatal(err)
	}
	
	opts := createClientOptions(uri)

	http.HandleFunc("/callback", func(w http.ResponseWriter, req *http.Request) {

		header := req.Header
		if params, ok := header["X-Forwarded-Proto"]; ok {
			if len(params) != 0 && params[0] == "http" {
				newURL := "https://" + req.Host + req.URL.Path
				http.Redirect(w, req, newURL, http.StatusMovedPermanently)
			}
		}

		events, err := bot.ParseRequest(req)
		if err != nil {
			if err == linebot.ErrInvalidSignature {
				w.WriteHeader(400)
			} else {
				w.WriteHeader(500)
			}
			return
		}
		for _, event := range events {
			if event.Type == linebot.EventTypeMessage {
				switch message := event.Message.(type) {
				case *linebot.TextMessage:
					var returnMessage string
					if message.Text == "暖房つけて" {
						returnMessage = "暖房つけました"
						sendMQTT(opts, "heatOn")
					} else if message.Text == "除湿つけて" {
						returnMessage = "除湿つけました"
						sendMQTT(opts, "defOn")
					} else if message.Text == "冷房つけて" {
						returnMessage = "冷房つけました"
						sendMQTT(opts, "airOn")
					} else if message.Text == "エアコンけして" {
						returnMessage = "けしました"
						sendMQTT(opts, "airconOff")
					} else {
						returnMessage = "理解できません"
					}
					if _, err = bot.ReplyMessage(
						event.ReplyToken,
						linebot.NewTextMessage(returnMessage),
					).Do(); err != nil {
						log.Print(err)
					}
				}
			}
		}
	})

	if err := http.ListenAndServe(":"+os.Getenv("PORT"), nil); err != nil {
		log.Fatal(err)
	}
}

func createClientOptions(uri *url.URL) *mqtt.ClientOptions {
	//TLSconfig
	certpool := x509.NewCertPool()
	severCert, err := ioutil.ReadFile("ca-certificates.crt")
	if err != nil {
		log.Fatal("Could not load server certificate!")
	}
	certpool.AppendCertsFromPEM(severCert)

	tlsconfig := &tls.Config{MinVersion: tls.VersionTLS12, ClientCAs: certpool}

	//MQTTclient options
	opts := mqtt.NewClientOptions()
	opts.AddBroker("ssl://" + uri.Host)
	opts.SetUsername(uri.User.Username())
	password, _ := uri.User.Password()
	opts.SetPassword(password)
	opts.SetClientID("LINE")
	opts.SetTLSConfig(tlsconfig)
	return opts
}

func sendMQTT(opts *mqtt.ClientOptions, sendMessage string) {
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	if token := client.Publish("esp32/aircon", 0, false,
		sendMessage); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	client.Disconnect(250)
}
