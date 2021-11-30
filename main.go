package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
)

type webhookReqBody struct {
	Message struct {
		Text string `json:"text"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
}

type sendMessageReqBody struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

// This handler is called everytime telegram sends us a webhook event
func Handler(res http.ResponseWriter, req *http.Request) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")

	// First, decode the JSON response body
	reqBody := &webhookReqBody{}
	if err := json.NewDecoder(req.Body).Decode(reqBody); err != nil {
		fmt.Printf("could not decode request body: %s\n", err)
		return
	}

	if err := makeRequest(reqBody.Message.Chat.ID, token); err != nil {
		fmt.Printf("could send response: %s\n", err)
		return
	}

	// log a confirmation message if the message is sent successfully
	fmt.Println("reply sent")
}

// Server listen to port 3000
func main() {

	port := os.Getenv("PORT")

	fmt.Printf("Listen to port %s\n", port)
	http.ListenAndServe(fmt.Sprintf(":%s", port), http.HandlerFunc(Handler))
}

func makeRequest(chatID int64, telegramToken string) error {
	resBody := sendMessageReqBody{
		ChatID: chatID,
		Text:   "HI!",
	}

	// Create the JSON body from the struct
	resBytes, err := json.Marshal(resBody)
	if err != nil {
		return err
	}

	// Send a post request with your token
	res, err := http.Post(fmt.Sprintf("https://api.telegram.org/%s/sendMessage", telegramToken), "application/json", bytes.NewBuffer(resBytes))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return errors.New("unexpected status " + res.Status)
	}

	return nil
}
