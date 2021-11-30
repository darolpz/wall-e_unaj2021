package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
)

const (
	sendMessage = "https://api.telegram.org/bot%s/sendMessage"
	getFile     = "https://api.telegram.org/bot%s/getFile?file_id=%s"
	file        = "https://api.telegram.org/file/bot%s/%s"
)

type webhookReqBody struct {
	UpdateID int     `json:"update_id"`
	Message  Message `json:"message"`
}

type Message struct {
	Photo []PhotoSize `json:"photo"`
	Text  string      `json:"text"`
	Chat  struct {
		ID int64 `json:"id"`
	} `json:"chat"`
}

type sendMessageReqBody struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

type PhotoSize struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	FileSize     int    `json:"file_size"`
}

type File struct {
	FileID   string `json:"file_id"`
	FilePath string `json:"file_path"`
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
	fmt.Printf("reqBody: %+v", reqBody)

	if err := makeRequest(reqBody.Message, token); err != nil {
		fmt.Printf("could send response: %s\n", err)
		return
	}

	// log a confirmation message if the message is sent successfully
	fmt.Println("reply sent")
}

// Server listen to port 3000
func main() {
	// Just for debugging
	/* err := godotenv.Load(".env")
	if err != nil {
		panic(err)
	} */
	port := os.Getenv("PORT")

	fmt.Printf("Listen to port %s\n", port)
	http.ListenAndServe(fmt.Sprintf(":%s", port), http.HandlerFunc(Handler))
}

func makeRequest(message Message, telegramToken string) error {
	resBody := sendMessageReqBody{
		ChatID: message.Chat.ID,
		Text:   "HI!",
	}

	// Create the JSON body from the struct
	resBytes, err := json.Marshal(resBody)
	if err != nil {
		return err
	}

	getFileRes, err := http.Get(fmt.Sprintf(getFile, telegramToken, message.Photo[0].FileID))
	if err != nil {
		fmt.Printf("getfile response error: %s\n", err)
		return err
	}
	defer getFileRes.Body.Close()
	file := &File{}
	if err := json.NewDecoder(getFileRes.Body).Decode(file); err != nil {
		fmt.Printf("could not decode file: %s\n", err)
		return err
	}
	fmt.Printf("File: %+v\n", file)

	// Send a post request with your token
	sendMessageRes, err := http.Post(fmt.Sprintf(sendMessage, telegramToken), "application/json", bytes.NewBuffer(resBytes))
	if err != nil {
		return err
	}
	defer sendMessageRes.Body.Close()

	if sendMessageRes.StatusCode != http.StatusOK {
		return errors.New("unexpected status " + sendMessageRes.Status)
	}

	return nil
}
