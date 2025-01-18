package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
	"net/http"
	"strconv"
	"strings"
)

var ErrNotValidSender = errors.New("неправильный исходящий кошелёк")
var ErrNotValidReceiver = errors.New("неправильный входящий кошелёк")
var ErrNotValidAmount = errors.New("неправильная сумма перевода")

// Структура используется для парсинга Send запроса
type sendRequest struct {
	Sender   string `json:"from"`
	Receiver string `json:"to"`
	Amount   string `json:"amount"`
}

// Функция совершает перевод между двумя кошельками. При успехе отправит http.StatusOK.
// При неудаче - ошибку.
func (app *application) send(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: http.StatusText(http.StatusInternalServerError)})
		return
	}

	defer r.Body.Close()

	var request *sendRequest
	err = json.Unmarshal(body, &request)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: http.StatusText(http.StatusBadRequest)})
		return
	}

	err = validateSendRequest(request)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	sender, _ := uuid.Parse(request.Sender)
	receiver, _ := uuid.Parse(request.Receiver)
	amount, _ := strconv.ParseFloat(request.Amount, 64)

	err = app.data.Send(sender, receiver, amount)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Функция получает число и возвращает такое количество транзакций.
func (app *application) getLast(w http.ResponseWriter, r *http.Request) {
	num, err := strconv.ParseInt(r.URL.Query().Get("count"), 10, 64)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: http.StatusText(http.StatusBadRequest)})
		return
	}

	txs, err := app.data.GetLast(int(num))
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: http.StatusText(http.StatusInternalServerError)})
		return
	}

	data, err := json.Marshal(txs)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: http.StatusText(http.StatusInternalServerError)})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// Функция получает id кошелька и возвращает информацию по нему.
func (app *application) getBalance(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: http.StatusText(http.StatusBadRequest)})
		return
	}

	id, err := uuid.Parse(parts[3])
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "неправильный кошелёк"})
		return
	}

	purse, err := app.data.GetBalance(id)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	data, err := json.Marshal(purse)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: http.StatusText(http.StatusInternalServerError)})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// Функция проверяет правильность кошелка отправителя и получателя,
// также проверяет сумму перевода на соответствие правилам
func validateSendRequest(request *sendRequest) error {
	if _, err := uuid.Parse(request.Sender); err != nil {
		return ErrNotValidSender
	}

	if _, err := uuid.Parse(request.Receiver); err != nil {
		return ErrNotValidReceiver
	}

	num, err := strconv.ParseFloat(request.Amount, 64)
	if err != nil {
		return ErrNotValidAmount
	}

	if num <= 0 {
		return ErrNotValidAmount
	}

	formatted := fmt.Sprintf("%.2f", num)

	if request.Amount != formatted {
		return ErrNotValidAmount
	}

	return nil
}
