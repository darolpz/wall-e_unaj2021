package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	sendMessageEndpoint = "https://api.telegram.org/bot%s/sendMessage"
	getFileEndpoint     = "https://api.telegram.org/bot%s/getFile?file_id=%s"
	fileEndpoint        = "https://api.telegram.org/file/bot%s/%s"
)

type webhookReqBody struct {
	UpdateID int     `json:"update_id"`
	Message  Message `json:"message"`
}

type Message struct {
	Photo    []PhotoSize `json:"photo"`
	Text     string      `json:"text"`
	Document Document    `json:"document"`
	Chat     struct {
		ID int64 `json:"id"`
	} `json:"chat"`
}

type Document struct {
	FileID string `json:"file_id"`
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

type FileResponse struct {
	OK     bool `json:"ok"`
	Result File `json:"result"`
}

type File struct {
	FileID   string `json:"file_id"`
	FilePath string `json:"file_path"`
}

type AWSResponse struct {
	Class       string `json:"class"`
	Probability string `json:"probability"`
}

var classesDictionary = map[string]string{
	"cardboard": "carton",
	"glass":     "vidrio",
	"metal":     "metal",
	"organic":   "organico",
	"paper":     "papel",
	"plastic":   "plastico",
	"trash":     "otros",
}

// ['cardboard', 'glass', 'metal', 'organic', 'paper', 'plastic', 'trash']

// This handler is called everytime telegram sends us a webhook event
func Handler(res http.ResponseWriter, req *http.Request) {
	telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN")

	// First, decode the JSON response body
	reqBody := &webhookReqBody{}
	if err := json.NewDecoder(req.Body).Decode(reqBody); err != nil {
		fmt.Printf("could not decode request body: %s\n", err)
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	fmt.Printf("reqBody: %+v\n", reqBody)

	if len(reqBody.Message.Photo) == 0 {
		fmt.Printf("Image not found\n")
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	fileID := reqBody.Message.Document.FileID
	lastPhoto := reqBody.Message.Photo[len(reqBody.Message.Photo)-1]

	if fileID != "" {
		fileID = lastPhoto.FileID
	}

	image, err := getFile(fileID, telegramToken)
	if err != nil {
		fmt.Printf("could not get image: %s\n", err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	awsRes, err := classify(image)
	if err != nil {
		fmt.Printf("could not classify image: %s\n", err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := makeRequest(reqBody.Message.Chat.ID, telegramToken, awsRes); err != nil {
		fmt.Printf("could send response: %s\n", err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusOK)
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

func makeRequest(chatID int64, telegramToken string, awsResponse AWSResponse) error {
	spanishClass, ok := classesDictionary[awsResponse.Class]
	if !ok {
		return errors.New("unknown class")
	}

	resBody := sendMessageReqBody{
		ChatID: chatID,
		Text:   fmt.Sprintf("Hay un %s de probabilidad que su residuo sea de tipo: %s", awsResponse.Probability, spanishClass),
	}

	// Create the JSON body from the struct
	resBytes, err := json.Marshal(resBody)
	if err != nil {
		fmt.Printf("could not marshal response: %s\n", err)
		return err
	}

	// Send a post request with your token
	sendMessageRes, err := http.Post(fmt.Sprintf(sendMessageEndpoint, telegramToken), "application/json", bytes.NewBuffer(resBytes))
	if err != nil {
		fmt.Printf("send message response error: %s\n", err)
		return err
	}
	defer sendMessageRes.Body.Close()

	if sendMessageRes.StatusCode != http.StatusOK {
		return errors.New("unexpected status " + sendMessageRes.Status)
	}

	return nil
}

func getFile(fileID, telegramToken string) ([]byte, error) {
	getFileRes, err := http.Get(fmt.Sprintf(getFileEndpoint, telegramToken, fileID))
	if err != nil {
		fmt.Printf("getfile response error: %s\n", err)
		return []byte{}, err
	}
	defer getFileRes.Body.Close()

	fileResponse := &FileResponse{}
	if err := json.NewDecoder(getFileRes.Body).Decode(fileResponse); err != nil {
		fmt.Printf("could not decode file: %s\n", err)
		return []byte{}, err
	}
	fmt.Printf("%v\n", fileResponse)

	fileRes, err := http.Get(fmt.Sprintf(fileEndpoint, telegramToken, fileResponse.Result.FilePath))
	if err != nil {
		fmt.Printf("file response error: %s\n", err)
		return []byte{}, err
	}
	defer fileRes.Body.Close()

	// We read all the bytes of the image
	// Types: data []byte
	data, err := ioutil.ReadAll(fileRes.Body)
	if err != nil {
		fmt.Printf("read file error: %s\n", err)
		return []byte{}, err
	}

	return data, nil
}

func classify(imageByte []byte) (AWSResponse, error) {
	awsEndpoint := os.Getenv("RECICLA_IA_ENDPOINT")
	req, err := http.NewRequest("POST", awsEndpoint, bytes.NewReader(imageByte))
	if err != nil {
		fmt.Printf("could not build request: %s\n", err)
		return AWSResponse{}, err
	}
	req.Header.Set("Content-Type", "image/jpeg")

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	rsp, err := client.Do(req)
	if err != nil {
		fmt.Printf("classify response error: %s\n", err)
		return AWSResponse{}, err
	}
	if rsp.StatusCode != http.StatusOK {
		log.Printf("classify request failed with response code: %d", rsp.StatusCode)
	}

	awsResponse := &AWSResponse{}
	if err := json.NewDecoder(rsp.Body).Decode(awsResponse); err != nil {
		fmt.Printf("could not decode file: %s\n", err)
		return AWSResponse{}, err
	}

	fmt.Printf("awsResponse: %v\n", awsResponse)
	return *awsResponse, nil
}
